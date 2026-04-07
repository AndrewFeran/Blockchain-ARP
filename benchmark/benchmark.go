package main

import (
	"crypto/x509"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ─── Configuration ────────────────────────────────────────────────────────────

var (
	mspID         = getEnv("MSP_ID", "Org1MSP")
	peerEndpoint  = getEnv("PEER_ENDPOINT", "localhost:7051")
	gatewayPeer   = getEnv("GATEWAY_PEER", "peer0.org1.example.com")
	channelName   = getEnv("CHANNEL_NAME", "mychannel")
	chaincodeName = getEnv("CHAINCODE_NAME", "arptracker")

	cryptoPath  = getEnv("CRYPTO_PATH", "/fabric-config/organizations/peerOrganizations/org1.example.com")
	certPath    = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"

	// Number of samples for each test.
	// Override via env vars if needed.
	writeTrials     = getEnvInt("BENCH_WRITE_TRIALS", 50)
	readTrials      = getEnvInt("BENCH_READ_TRIALS", 100)
	readAllTrials   = getEnvInt("BENCH_READ_ALL_TRIALS", 20)
	throughputCount = getEnvInt("BENCH_THROUGHPUT_COUNT", 100)
	concurrency     = getEnvInt("BENCH_CONCURRENCY", 10)
	warmupCount     = getEnvInt("BENCH_WARMUP", 5)
)

// ─── Result types ─────────────────────────────────────────────────────────────

type Stats struct {
	Count  int     `json:"count"`
	MinMs  float64 `json:"min_ms"`
	MaxMs  float64 `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
	P50Ms  float64 `json:"p50_ms"`
	P95Ms  float64 `json:"p95_ms"`
	P99Ms  float64 `json:"p99_ms"`
	StdDev float64 `json:"stddev_ms"`
}

type BenchmarkResults struct {
	Timestamp        string  `json:"timestamp"`
	PeerEndpoint     string  `json:"peer_endpoint"`
	WriteLatency     Stats   `json:"write_latency"`
	ReadSingleLatency Stats  `json:"read_single_latency"`
	ReadAllLatency   Stats   `json:"read_all_latency"`
	SeqThroughputTPS float64 `json:"sequential_throughput_tps"`
	ConcThroughputTPS float64 `json:"concurrent_throughput_tps"`
	ConcurrentWorkers int    `json:"concurrent_workers"`
	WriteTrials      int     `json:"write_trials"`
	ReadTrials       int     `json:"read_trials"`
	ThroughputCount  int     `json:"throughput_count"`
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	log.Println("============================================================")
	log.Println("  📊  HYPERLEDGER FABRIC ARP BENCHMARK")
	log.Println("============================================================")
	log.Printf("Peer:       %s (%s)\n", gatewayPeer, peerEndpoint)
	log.Printf("Channel:    %s   Chaincode: %s\n", channelName, chaincodeName)
	log.Printf("Write trials:      %d\n", writeTrials)
	log.Printf("Read trials:       %d\n", readTrials)
	log.Printf("Read-all trials:   %d\n", readAllTrials)
	log.Printf("Throughput count:  %d (seq) / %d (conc, workers=%d)\n",
		throughputCount, throughputCount, concurrency)
	log.Println("============================================================")
	log.Println()

	// Connect
	conn := newGrpcConnection()
	defer conn.Close()

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
		log.Fatalf("Failed to connect to Fabric gateway: %v", err)
	}
	defer gw.Close()

	contract := gw.GetNetwork(channelName).GetContract(chaincodeName)

	// Warmup: submit a few transactions to prime the connection; results discarded.
	log.Printf("🔥 Warming up (%d transactions, results discarded)...\n", warmupCount)
	for i := 0; i < warmupCount; i++ {
		submitWrite(contract, fmt.Sprintf("10.0.0.%d", i))
	}
	log.Println()

	results := BenchmarkResults{
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		PeerEndpoint:      peerEndpoint,
		ConcurrentWorkers: concurrency,
		WriteTrials:       writeTrials,
		ReadTrials:        readTrials,
		ThroughputCount:   throughputCount,
	}

	// Collect raw samples for CSV export.
	allSamples := map[string][]float64{}

	// ── 1. Write latency ────────────────────────────────────────────────────
	log.Printf("📝  [1/5] Write latency  (%d transactions)...\n", writeTrials)
	writeSamples := benchWriteLatency(contract, writeTrials, "10.99")
	results.WriteLatency = computeStats(writeSamples)
	allSamples["write_latency_ms"] = writeSamples
	printStats("Write Latency", results.WriteLatency)

	// ── 2. Read (single entry) latency ──────────────────────────────────────
	log.Printf("📖  [2/5] Read (single entry) latency  (%d queries)...\n", readTrials)
	readSamples := benchReadSingleLatency(contract, readTrials, "10.99")
	results.ReadSingleLatency = computeStats(readSamples)
	allSamples["read_single_latency_ms"] = readSamples
	printStats("Read (single) Latency", results.ReadSingleLatency)

	// ── 3. Read (all entries) latency ───────────────────────────────────────
	log.Printf("📖  [3/5] Read (all entries) latency  (%d queries)...\n", readAllTrials)
	readAllSamples := benchReadAllLatency(contract, readAllTrials)
	results.ReadAllLatency = computeStats(readAllSamples)
	allSamples["read_all_latency_ms"] = readAllSamples
	printStats("Read (all entries) Latency", results.ReadAllLatency)

	// ── 4. Sequential throughput ────────────────────────────────────────────
	log.Printf("⚡  [4/5] Sequential throughput  (%d transactions)...\n", throughputCount)
	results.SeqThroughputTPS = benchSeqThroughput(contract, throughputCount, "10.88")
	log.Printf("   ➜  Sequential TPS: %.2f\n\n", results.SeqThroughputTPS)

	// ── 5. Concurrent throughput ────────────────────────────────────────────
	log.Printf("🚀  [5/5] Concurrent throughput  (%d transactions, %d workers)...\n",
		throughputCount, concurrency)
	results.ConcThroughputTPS = benchConcThroughput(contract, throughputCount, concurrency, "10.77")
	log.Printf("   ➜  Concurrent TPS: %.2f\n\n", results.ConcThroughputTPS)

	// ── Output ──────────────────────────────────────────────────────────────
	writeResults(results, allSamples)
}

// ─── Benchmark functions ──────────────────────────────────────────────────────

// benchWriteLatency submits writeTrials individual transactions and records
// the wall-clock latency of each SubmitTransaction call (proposal → endorse →
// order → commit).  Uses IPs in the subnet prefix "10.99.x.x".
func benchWriteLatency(contract *client.Contract, n int, prefix string) []float64 {
	samples := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		ip := fmt.Sprintf("%s.%d.%d", prefix, i/256, i%256)
		start := time.Now()
		err := submitWrite(contract, ip)
		ms := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			log.Printf("   ⚠️  Write %d failed: %v", i, err)
			continue
		}
		samples = append(samples, ms)
		if (i+1)%10 == 0 {
			log.Printf("   Progress: %d/%d  (last: %.1f ms)", i+1, n, ms)
		}
	}
	log.Println()
	return samples
}

// benchReadSingleLatency evaluates GetCurrentARPEntry for IPs written in the
// write latency phase.
func benchReadSingleLatency(contract *client.Contract, n int, prefix string) []float64 {
	samples := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		// Cycle through the IPs written during the write phase.
		idx := i % writeTrials
		ip := fmt.Sprintf("%s.%d.%d", prefix, idx/256, idx%256)
		start := time.Now()
		_, err := contract.EvaluateTransaction("GetCurrentARPEntry", ip)
		ms := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			log.Printf("   ⚠️  Read %d failed: %v", i, err)
			continue
		}
		samples = append(samples, ms)
		if (i+1)%20 == 0 {
			log.Printf("   Progress: %d/%d  (last: %.1f ms)", i+1, n, ms)
		}
	}
	log.Println()
	return samples
}

// benchReadAllLatency evaluates GetAllARPEntries (full ledger scan) n times.
func benchReadAllLatency(contract *client.Contract, n int) []float64 {
	samples := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		_, err := contract.EvaluateTransaction("GetAllARPEntries")
		ms := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			log.Printf("   ⚠️  ReadAll %d failed: %v", i, err)
			continue
		}
		samples = append(samples, ms)
		log.Printf("   Progress: %d/%d  (last: %.1f ms)", i+1, n, ms)
	}
	log.Println()
	return samples
}

// benchSeqThroughput submits n transactions serially and returns TPS.
func benchSeqThroughput(contract *client.Contract, n int, prefix string) float64 {
	succeeded := 0
	start := time.Now()
	for i := 0; i < n; i++ {
		ip := fmt.Sprintf("%s.%d.%d", prefix, i/256, i%256)
		if err := submitWrite(contract, ip); err != nil {
			log.Printf("   ⚠️  Tx %d failed: %v", i, err)
		} else {
			succeeded++
		}
	}
	elapsed := time.Since(start).Seconds()
	log.Printf("   Completed %d/%d in %.2f s\n", succeeded, n, elapsed)
	return float64(succeeded) / elapsed
}

// benchConcThroughput submits n transactions across `workers` goroutines and
// returns TPS measured as wall-clock throughput.
func benchConcThroughput(contract *client.Contract, n, workers int, prefix string) float64 {
	jobs := make(chan int, n)
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)

	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range jobs {
				base := workerID*1000 + i
				ip := fmt.Sprintf("%s.%d.%d", prefix, base/256, base%256)
				if err := submitWrite(contract, ip); err != nil {
					log.Printf("   ⚠️  Worker %d tx %d failed: %v", workerID, i, err)
				} else {
					mu.Lock()
					succeeded++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	log.Printf("   Completed %d/%d in %.2f s\n", succeeded, n, elapsed)
	return float64(succeeded) / elapsed
}

// submitWrite records a single ARP entry on the ledger and returns any error.
func submitWrite(contract *client.Contract, ip string) error {
	mac := ipToMAC(ip)
	_, err := contract.SubmitTransaction("RecordARPEntry",
		ip, mac, "bench0", "benchmark", "dynamic", "reachable", "benchmark")
	return err
}

// ipToMAC derives a deterministic MAC from an IP string so each write uses a
// unique, stable MAC that will always register as event type "match" on repeats.
func ipToMAC(ip string) string {
	h := uint32(0)
	for _, c := range ip {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("be:bc:%02x:%02x:%02x:%02x",
		(h>>24)&0xff, (h>>16)&0xff, (h>>8)&0xff, h&0xff)
}

// ─── Statistics ───────────────────────────────────────────────────────────────

func computeStats(samples []float64) Stats {
	if len(samples) == 0 {
		return Stats{}
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	n := len(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(n)

	variance := 0.0
	for _, v := range sorted {
		d := v - mean
		variance += d * d
	}
	variance /= float64(n)

	return Stats{
		Count:  n,
		MinMs:  sorted[0],
		MaxMs:  sorted[n-1],
		MeanMs: mean,
		P50Ms:  percentile(sorted, 50),
		P95Ms:  percentile(sorted, 95),
		P99Ms:  percentile(sorted, 99),
		StdDev: math.Sqrt(variance),
	}
}

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func printStats(name string, s Stats) {
	log.Printf("   ── %s (n=%d) ──\n", name, s.Count)
	log.Printf("   Min: %8.2f ms  Max: %8.2f ms  Mean: %8.2f ms  StdDev: %6.2f ms\n",
		s.MinMs, s.MaxMs, s.MeanMs, s.StdDev)
	log.Printf("   P50: %8.2f ms  P95: %8.2f ms   P99: %8.2f ms\n\n",
		s.P50Ms, s.P95Ms, s.P99Ms)
}

// ─── Output ───────────────────────────────────────────────────────────────────

func writeResults(results BenchmarkResults, allSamples map[string][]float64) {
	outDir := getEnv("RESULTS_DIR", "/results")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("⚠️  Cannot create results dir %s: %v — printing to stdout", outDir, err)
		outDir = ""
	}

	// JSON summary
	jsonBytes, _ := json.MarshalIndent(results, "", "  ")
	if outDir != "" {
		path := filepath.Join(outDir, "benchmark_results.json")
		if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
			log.Printf("⚠️  Could not write JSON: %v", err)
		} else {
			log.Printf("✅  Results JSON  → %s", path)
		}
	} else {
		fmt.Println(string(jsonBytes))
	}

	// CSV of raw latency samples — useful for plotting in the paper
	if outDir != "" {
		path := filepath.Join(outDir, "latency_samples.csv")
		f, err := os.Create(path)
		if err != nil {
			log.Printf("⚠️  Could not create CSV: %v", err)
			return
		}
		defer f.Close()
		w := csv.NewWriter(f)

		// Header: one column per test series
		keys := []string{"write_latency_ms", "read_single_latency_ms", "read_all_latency_ms"}
		w.Write(keys)

		// Find max length
		maxLen := 0
		for _, k := range keys {
			if len(allSamples[k]) > maxLen {
				maxLen = len(allSamples[k])
			}
		}
		for i := 0; i < maxLen; i++ {
			row := make([]string, len(keys))
			for j, k := range keys {
				if i < len(allSamples[k]) {
					row[j] = strconv.FormatFloat(allSamples[k][i], 'f', 3, 64)
				}
			}
			w.Write(row)
		}
		w.Flush()
		log.Printf("✅  Latency CSV   → %s", path)
	}

	// Final summary to console
	log.Println()
	log.Println("============================================================")
	log.Println("  SUMMARY")
	log.Println("============================================================")
	log.Printf("  Write latency        mean: %7.2f ms  p95: %7.2f ms  p99: %7.2f ms",
		results.WriteLatency.MeanMs, results.WriteLatency.P95Ms, results.WriteLatency.P99Ms)
	log.Printf("  Read (single) latency mean: %6.2f ms  p95: %7.2f ms  p99: %7.2f ms",
		results.ReadSingleLatency.MeanMs, results.ReadSingleLatency.P95Ms, results.ReadSingleLatency.P99Ms)
	log.Printf("  Read (all) latency   mean: %7.2f ms  p95: %7.2f ms  p99: %7.2f ms",
		results.ReadAllLatency.MeanMs, results.ReadAllLatency.P95Ms, results.ReadAllLatency.P99Ms)
	log.Printf("  Sequential TPS:    %.2f", results.SeqThroughputTPS)
	log.Printf("  Concurrent TPS:    %.2f  (%d workers)", results.ConcThroughputTPS, results.ConcurrentWorkers)
	log.Println("============================================================")
}

// ─── Fabric connection helpers (mirrors router-monitor.go) ────────────────────

func newGrpcConnection() *grpc.ClientConn {
	cert, err := loadCertificate(tlsCertPath)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	creds := credentials.NewClientTLSFromCert(certPool, gatewayPeer)
	conn, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("Failed to create gRPC connection: %v", err)
	}
	return conn
}

func newIdentity() *identity.X509Identity {
	cert, err := loadCertificate(certPath)
	if err != nil {
		log.Fatalf("Failed to load certificate: %v", err)
	}
	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		log.Fatalf("Failed to create identity: %v", err)
	}
	return id
}

func newSign() identity.Sign {
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		log.Fatalf("Failed to read private key directory: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("No private key found in %s", keyPath)
	}
	privateKeyPEM, err := ioutil.ReadFile(filepath.Join(keyPath, files[0].Name()))
	if err != nil {
		log.Fatalf("Failed to read private key file: %v", err)
	}
	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
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

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
