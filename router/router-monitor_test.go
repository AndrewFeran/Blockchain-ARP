package main

import (
	"testing"

	"github.com/google/gopacket/layers"
)

func TestParseARPObservationFormatsPacketFields(t *testing.T) {
	observation := parseARPObservation(&layers.ARP{
		Operation:         2,
		SourceProtAddress: []byte{10, 5, 0, 99},
		SourceHwAddress:   []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x99},
		DstProtAddress:    []byte{10, 5, 0, 10},
	})

	if observation.opStr != "Reply" {
		t.Fatalf("expected Reply operation, got %q", observation.opStr)
	}
	if observation.srcIP != "10.5.0.99" {
		t.Fatalf("unexpected source IP: %s", observation.srcIP)
	}
	if observation.srcMAC != "aa:bb:cc:dd:ee:99" {
		t.Fatalf("unexpected source MAC: %s", observation.srcMAC)
	}
	if observation.dstIP != "10.5.0.10" {
		t.Fatalf("unexpected destination IP: %s", observation.dstIP)
	}
}

func TestParseARPObservationLabelsUnknownOperation(t *testing.T) {
	observation := parseARPObservation(&layers.ARP{
		Operation:         99,
		SourceProtAddress: []byte{10, 5, 0, 1},
		SourceHwAddress:   []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstProtAddress:    []byte{10, 5, 0, 10},
	})

	if observation.opStr != "Unknown" {
		t.Fatalf("expected Unknown operation, got %q", observation.opStr)
	}
}
