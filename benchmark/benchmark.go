package main

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	mspID         = getEnv("MSP_ID", "Org1MSP")
	peerEndpoint  = getEnv("PEER_ENDPOINT", "localhost:7051")
	gatewayPeer   = getEnv("GATEWAY_PEER", "peer0.org1.example.com")
	channelName   = getEnv("CHANNEL_NAME", "mychannel")
	chaincodeName = getEnv("CHAINCODE_NAME", "arptracker")
	resultsDir    = getEnv("RESULTS_DIR", "results")

	cryptoPath  = getEnv("CRYPTO_PATH", "/fabric-config/organizations/peerOrganizations/org1.example.com")
	certPath    = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
)

type fabricSession struct {
	gateway  *client.Gateway
	contract *client.Contract
	conn     *grpc.ClientConn
}

type statSummary struct {
	Count  int
	MeanMs float64
	P50Ms  float64
	P95Ms  float64
	P99Ms  float64
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		log.Fatalf("failed to create results directory: %v", err)
	}

	session := connectToFabric()
	defer session.close()

	var err error
	switch os.Args[1] {
	case "latency":
		err = runLatency(session.contract, os.Args[2:])
	case "throughput":
		err = runThroughput(session.contract, os.Args[2:])
	case "coldstart":
		err = runColdStart(session.contract, os.Args[2:])
	case "baseline":
		err = runBaseline(session.contract, os.Args[2:])
	case "all":
		err = runLatency(session.contract, []string{"--nodes", "5,10,15,20", "--trials", "30"})
		if err == nil {
			err = runThroughput(session.contract, []string{"--max-rate", "100"})
		}
		if err == nil {
			err = runColdStart(session.contract, []string{"--ledger-sizes", "10,50,100,250", "--trials", "10"})
		}
		if err == nil {
			err = runBaseline(session.contract, []string{"--trials", "10"})
		}
		if err == nil {
			err = runAnalyzer()
		}
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	log.Println("Run with:")
	log.Println("  go run benchmark.go latency --nodes 5,10,15,20 --trials 30")
	log.Println("  go run benchmark.go throughput --max-rate 100")
	log.Println("  go run benchmark.go coldstart --ledger-sizes 10,50,100,250 --trials 10")
	log.Println("  go run benchmark.go baseline --trials 10")
	log.Println("  go run benchmark.go all")
}

func runLatency(contract *client.Contract, args []string) error {
	nodesCSV, trials, err := parseCommonFlags(args, "5,10,15,20", 30)
	if err != nil {
		return err
	}
	nodes, err := parseIntList(nodesCSV)
	if err != nil {
		return err
	}

	path := resultPath("latency")
	file, writer, err := newCSV(path, []string{"node_count", "trial", "trial_id", "ip", "mac", "submit_latency_ms", "detection_latency_ms", "nodes_observed"})
	if err != nil {
		return err
	}
	defer file.Close()

	for _, nodeCount := range nodes {
		samples := make([]float64, 0, trials)
		for trial := 1; trial <= trials; trial++ {
			ip := fmt.Sprintf("10.250.%d.%d", nodeCount+trial/256, trial%256)
			mac := deterministicMAC("latency", nodeCount, trial)
			started := time.Now()
			trialID, elapsed, err := submitARPEvent(contract, ip, mac, "benchmark-latency")
			if err != nil {
				return err
			}
			submitMS := elapsedMs(elapsed)
			detectionMS, observed := waitForNodeCacheUpdate(trialID, activeNodeLimit(nodeCount, runningLanNodeCount()), started)
			samples = append(samples, detectionMS)
			writer.Write([]string{itoa(nodeCount), itoa(trial), trialID, ip, mac, ftoa(submitMS), ftoa(detectionMS), itoa(observed)})
		}
		printSummary(fmt.Sprintf("latency nodes=%d", nodeCount), summarize(samples))
	}
	writer.Flush()
	log.Printf("wrote %s", path)
	return writer.Error()
}

func runThroughput(contract *client.Contract, args []string) error {
	fs := newFlagSet("throughput")
	maxRate := fs.Int("max-rate", 100, "maximum ARP events per second")
	step := fs.Int("step", 10, "rate step")
	events := fs.Int("events", 10, "events per rate")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := resultPath("throughput")
	file, writer, err := newCSV(path, []string{"rate_per_sec", "event", "trial_id", "ip", "mac", "submit_latency_ms", "detection_latency_ms", "nodes_observed"})
	if err != nil {
		return err
	}
	defer file.Close()

	for rate := 1; rate <= *maxRate; rate += *step {
		interval := time.Second / time.Duration(rate)
		samples := make([]float64, 0, *events)
		for event := 1; event <= *events; event++ {
			start := time.Now()
			ip := fmt.Sprintf("10.251.%d.%d", rate, event)
			mac := deterministicMAC("throughput", rate, event)
			started := time.Now()
			trialID, elapsed, err := submitARPEvent(contract, ip, mac, "benchmark-throughput")
			if err != nil {
				return err
			}
			submitMS := elapsedMs(elapsed)
			detectionMS, observed := waitForNodeCacheUpdate(trialID, runningLanNodeCount(), started)
			samples = append(samples, detectionMS)
			writer.Write([]string{itoa(rate), itoa(event), trialID, ip, mac, ftoa(submitMS), ftoa(detectionMS), itoa(observed)})
			if remaining := interval - time.Since(start); remaining > 0 {
				time.Sleep(remaining)
			}
		}
		printSummary(fmt.Sprintf("throughput rate=%d/s", rate), summarize(samples))
	}
	writer.Flush()
	log.Printf("wrote %s", path)
	return writer.Error()
}

func runColdStart(contract *client.Contract, args []string) error {
	sizesCSV, trials, err := parseCommonFlags(args, "10,50,100,250", 10)
	if err != nil {
		return err
	}
	sizes, err := parseIntList(sizesCSV)
	if err != nil {
		return err
	}

	path := resultPath("coldstart")
	file, writer, err := newCSV(path, []string{"ledger_size", "trial", "query_elapsed_ms", "simulated_install_elapsed_ms", "entries"})
	if err != nil {
		return err
	}
	defer file.Close()

	for _, size := range sizes {
		if err := prepopulateLedger(contract, size); err != nil {
			return err
		}
		samples := make([]float64, 0, trials)
		for trial := 1; trial <= trials; trial++ {
			start := time.Now()
			result, err := contract.EvaluateTransaction("GetAllARPEntries")
			if err != nil {
				return err
			}
			queryMS := elapsedMs(time.Since(start))
			entries := countJSONEntries(result)
			installMS := estimateColdStartInstallMS(queryMS, entries)
			samples = append(samples, installMS)
			writer.Write([]string{itoa(size), itoa(trial), ftoa(queryMS), ftoa(installMS), itoa(entries)})
		}
		printSummary(fmt.Sprintf("coldstart ledger=%d", size), summarize(samples))
	}
	writer.Flush()
	log.Printf("wrote %s", path)
	return writer.Error()
}

func runBaseline(contract *client.Contract, args []string) error {
	_, trials, err := parseCommonFlags(args, "", 10)
	if err != nil {
		return err
	}

	path := resultPath("baseline")
	file, writer, err := newCSV(path, []string{"trial", "baseline_poison_ms", "protected_submit_ms", "protected_detection_ms", "protected_rejected"})
	if err != nil {
		return err
	}
	defer file.Close()

	baselinePoisonMs := getEnvFloat("BENCH_BASELINE_POISON_MS", 50)
	samples := make([]float64, 0, trials)
	for trial := 1; trial <= trials; trial++ {
		ip := fmt.Sprintf("10.252.%d.%d", trial/256, trial%256)
		mac := deterministicMAC("baseline", trial, 1)
		started := time.Now()
		trialID, elapsed, err := submitARPEvent(contract, ip, mac, "benchmark-baseline")
		if err != nil {
			return err
		}
		submitMS := elapsedMs(elapsed)
		detectionMS, observed := waitForNodeCacheUpdate(trialID, runningLanNodeCount(), started)
		rejected := "0"
		if observed > 0 {
			rejected = "1"
		}
		samples = append(samples, detectionMS)
		writer.Write([]string{itoa(trial), ftoa(baselinePoisonMs), ftoa(submitMS), ftoa(detectionMS), rejected})
	}
	writer.Flush()
	printSummary("baseline protected path", summarize(samples))
	log.Printf("wrote %s", path)
	return writer.Error()
}

func parseCommonFlags(args []string, defaultList string, defaultTrials int) (string, int, error) {
	fs := newFlagSet("benchmark")
	nodes := fs.String("nodes", defaultList, "comma-separated node counts")
	sizes := fs.String("ledger-sizes", defaultList, "comma-separated ledger sizes")
	trials := fs.Int("trials", defaultTrials, "trial count")
	if err := fs.Parse(args); err != nil {
		return "", 0, err
	}
	if defaultList == "" {
		return "", *trials, nil
	}
	if strings.Contains(strings.Join(args, " "), "ledger-sizes") {
		return *sizes, *trials, nil
	}
	return *nodes, *trials, nil
}

func prepopulateLedger(contract *client.Contract, size int) error {
	for i := 1; i <= size; i++ {
		ip := fmt.Sprintf("10.253.%d.%d", i/256, i%256)
		mac := deterministicMAC("coldstart", size, i)
		if _, _, err := submitARPEvent(contract, ip, mac, "benchmark-coldstart"); err != nil {
			return err
		}
	}
	return nil
}

func submitARPEvent(contract *client.Contract, ip, mac, hostname string) (string, time.Duration, error) {
	trialID := newTrialID()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	_, err := contract.SubmitWithContext(
		ctx,
		"RecordARPEntry",
		client.WithArguments(ip, mac, "bench0", hostname, "dynamic", "reachable", "benchmark"),
		client.WithTransient(map[string][]byte{"trialID": []byte(trialID)}),
	)
	return trialID, time.Since(start), err
}

func waitForNodeCacheUpdate(trialID string, expectedNodes int, since time.Time) (float64, int) {
	if expectedNodes <= 0 {
		return 0, 0
	}

	deadline := time.Now().Add(getEnvDuration("BENCH_EVENT_TIMEOUT", 20*time.Second))
	seen := make(map[string]int64)
	for time.Now().Before(deadline) {
		for container, ts := range readNodeCacheUpdates(trialID, since) {
			seen[container] = ts
		}
		if len(seen) >= expectedNodes {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if len(seen) == 0 {
		return elapsedMs(time.Since(since)), 0
	}

	maxTS := int64(0)
	for _, ts := range seen {
		if ts > maxTS {
			maxTS = ts
		}
	}
	return float64(maxTS - since.UnixMilli()), len(seen)
}

func readNodeCacheUpdates(trialID string, since time.Time) map[string]int64 {
	updates := make(map[string]int64)
	for _, container := range runningLanNodeContainers() {
		output, err := dockerLogsSince(container, since)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(output, "\n") {
			if !strings.Contains(line, "BENCHMARK_EVENT") ||
				!strings.Contains(line, "stage=node_cache_updated") ||
				!strings.Contains(line, "trial="+trialID) {
				continue
			}
			if ts, ok := parseBenchmarkTimestamp(line); ok {
				updates[container] = ts
			}
		}
	}
	return updates
}

func runningLanNodeContainers() []string {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	containers := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSpace(line)
		if regexp.MustCompile(`^lan-node-[0-9]+$`).MatchString(name) {
			containers = append(containers, name)
		}
	}
	sort.Slice(containers, func(i, j int) bool {
		return nodeOrdinal(containers[i]) < nodeOrdinal(containers[j])
	})
	return containers
}

func runningLanNodeCount() int {
	return len(runningLanNodeContainers())
}

func nodeOrdinal(name string) int {
	parts := strings.Split(name, "-")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func dockerLogsSince(container string, since time.Time) (string, error) {
	cmd := exec.Command("docker", "logs", "--since", since.Add(-2*time.Second).UTC().Format(time.RFC3339), container)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

var benchmarkTimestampPattern = regexp.MustCompile(`\bts=([0-9]+)\b`)

func parseBenchmarkTimestamp(line string) (int64, bool) {
	matches := benchmarkTimestampPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return 0, false
	}
	ts, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}

func activeNodeLimit(requested, available int) int {
	if requested < 1 {
		return 1
	}
	if available < 1 {
		return 0
	}
	if requested > available {
		return available
	}
	return requested
}

func estimateColdStartInstallMS(queryMS float64, entries int) float64 {
	// Cold-start in the live node includes the ledger query plus one ip-neigh
	// replacement per active LAN entry. The replacement cost is deliberately
	// conservative so the CSV reflects the full bootstrap path, not just Fabric IO.
	return queryMS + float64(entries)*getEnvFloat("BENCH_ARP_INSTALL_MS", 2)
}

func runAnalyzer() error {
	cmd := exec.Command("python3", "analyze.py")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("python", "analyze.py")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func connectToFabric() *fabricSession {
	conn := newGrpcConnection()
	gw, err := client.Connect(
		newIdentity(),
		client.WithSign(newSign()),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(10*time.Second),
		client.WithEndorseTimeout(30*time.Second),
		client.WithSubmitTimeout(10*time.Second),
		client.WithCommitStatusTimeout(2*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to connect to Fabric gateway: %v", err)
	}

	return &fabricSession{
		gateway:  gw,
		contract: gw.GetNetwork(channelName).GetContract(chaincodeName),
		conn:     conn,
	}
}

func (s *fabricSession) close() {
	s.gateway.Close()
	s.conn.Close()
}

func newGrpcConnection() *grpc.ClientConn {
	cert, err := loadCertificate(tlsCertPath)
	if err != nil {
		log.Fatalf("failed to load TLS certificate: %v", err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	creds := credentials.NewClientTLSFromCert(certPool, gatewayPeer)
	conn, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("failed to create gRPC connection: %v", err)
	}
	return conn
}

func newIdentity() *identity.X509Identity {
	cert, err := loadCertificate(certPath)
	if err != nil {
		log.Fatalf("failed to load certificate: %v", err)
	}
	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		log.Fatalf("failed to create identity: %v", err)
	}
	return id
}

func newSign() identity.Sign {
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		log.Fatalf("failed to read private key directory: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no private key found in %s", keyPath)
	}
	privateKeyPEM, err := ioutil.ReadFile(filepath.Join(keyPath, files[0].Name()))
	if err != nil {
		log.Fatalf("failed to read private key file: %v", err)
	}
	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}
	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		log.Fatalf("failed to create signer: %v", err)
	}
	return sign
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	pem, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(pem)
}

func newCSV(path string, headers []string) (*os.File, *csv.Writer, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(headers); err != nil {
		file.Close()
		return nil, nil, err
	}
	return file, writer, nil
}

func resultPath(metric string) string {
	return filepath.Join(resultsDir, fmt.Sprintf("%s_%s.csv", metric, time.Now().UTC().Format("20060102T150405Z")))
}

func summarize(samples []float64) statSummary {
	if len(samples) == 0 {
		return statSummary{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, sample := range sorted {
		sum += sample
	}
	return statSummary{
		Count:  len(sorted),
		MeanMs: sum / float64(len(sorted)),
		P50Ms:  percentile(sorted, 50),
		P95Ms:  percentile(sorted, 95),
		P99Ms:  percentile(sorted, 99),
	}
}

func percentile(sorted []float64, p float64) float64 {
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func printSummary(label string, summary statSummary) {
	log.Printf("%s: n=%d mean=%.2fms p50=%.2fms p95=%.2fms p99=%.2fms",
		label, summary.Count, summary.MeanMs, summary.P50Ms, summary.P95Ms, summary.P99Ms)
}

func parseIntList(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func countJSONEntries(result []byte) int {
	trimmed := strings.TrimSpace(string(result))
	if trimmed == "[]" || trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "ipAddress")
}

func deterministicMAC(parts ...interface{}) string {
	seed := uint32(2166136261)
	for _, part := range parts {
		for _, r := range fmt.Sprint(part) {
			seed ^= uint32(r)
			seed *= 16777619
		}
	}
	return fmt.Sprintf("be:bc:%02x:%02x:%02x:%02x",
		(seed>>24)&0xff, (seed>>16)&0xff, (seed>>8)&0xff, seed&0xff)
}

func newTrialID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func newFlagSet(name string) *flagSet {
	return &flagSet{name: name}
}

type flagSet struct {
	name    string
	values  []flagValue
	parsers []func(string) error
}

type flagValue struct {
	name  string
	value string
}

func (f *flagSet) String(name, fallback, _ string) *string {
	value := fallback
	f.parsers = append(f.parsers, func(arg string) error {
		if strings.HasPrefix(arg, "--"+name+"=") {
			value = strings.TrimPrefix(arg, "--"+name+"=")
		}
		return nil
	})
	return &value
}

func (f *flagSet) Int(name string, fallback int, _ string) *int {
	value := fallback
	f.parsers = append(f.parsers, func(arg string) error {
		if strings.HasPrefix(arg, "--"+name+"=") {
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--"+name+"="))
			if err != nil {
				return err
			}
			value = n
		}
		return nil
	})
	return &value
}

func (f *flagSet) Parse(args []string) error {
	expanded := expandFlagArgs(args)
	for _, arg := range expanded {
		for _, parser := range f.parsers {
			if err := parser(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func expandFlagArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && !strings.Contains(args[i], "=") && i+1 < len(args) {
			out = append(out, args[i]+"="+args[i+1])
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func elapsedMs(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func ftoa(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
