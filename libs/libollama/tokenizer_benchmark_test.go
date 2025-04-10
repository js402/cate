package libollama_test

import (
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/js402/CATE/libs/libollama"
)

// Helper function to report operations per second
func showOpsPerSecond(b *testing.B, ops int64) {
	b.Helper()
	elapsed := b.Elapsed().Seconds()
	if elapsed > 0 {
		opsPerSec := float64(ops) / elapsed
		b.ReportMetric(opsPerSec, "ops/s") // Report custom ops/s metric
	}
}

// thanks to https://stackoverflow.com/questions/31663229/how-can-stdout-be-captured-or-suppressed-for-golang-testing
func quiet() func() {
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	serr := os.Stderr
	os.Stdout = null
	os.Stderr = null
	log.SetOutput(null)
	return func() {
		defer null.Close()
		os.Stdout = sout
		os.Stderr = serr
		log.SetOutput(os.Stderr)
	}
}

// Creates a tokenizer instance for benchmarking with preloaded 'tiny' model
func createBenchTokenizer(b *testing.B, model string) libollama.Tokenizer {
	b.Helper() // Mark as a benchmark helper
	httpClient := &http.Client{Timeout: 30 * time.Second}
	// Preload the 'tiny' model to exclude download/load time from benchmark loop
	tokenizer, err := libollama.NewTokenizer(
		libollama.TokenizerWithHTTPClient(httpClient),
		libollama.TokenizerWithPreloadedModels(model),
		libollama.TokenizerWithFallbackModel(model),
	)
	if err != nil {
		b.Fatalf("failed to initialize tokenizer: %v", err)
	}
	return tokenizer
}

func benchmarkTokenize(b *testing.B, size int, parallel bool, model string) {
	b.Helper()
	defer quiet()()

	tokenizer, err := libollama.NewTokenizer(
		libollama.TokenizerWithPreloadedModels(model),
	)
	if err != nil {
		b.Fatalf("failed to create tokenizer: %v", err)
	}

	input := strings.Repeat("a", size)
	b.SetBytes(int64(size))
	b.ReportAllocs()

	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := tokenizer.Tokenize(model, input); err != nil {
					b.Errorf("parallel tokenization error: %v", err)
				}
			}
		})
	} else {
		for b.Loop() {
			if _, err := tokenizer.Tokenize(model, input); err != nil {
				b.Fatalf("sequential tokenization error: %v", err)
			}
		}
	}
	b.StopTimer()
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkTokenizeSequential_4KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 4096, false, "tiny") // 1KB input
}

func BenchmarkTokenizeSequential_1KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 1024, false, "tiny") // 1KB input
}

func BenchmarkTokenizeSequential_0_01KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 10, false, "tiny") // 0.01KB input
}

func BenchmarkTokenizeSequential_0_001KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 1, false, "tiny") // 0.001KB input
}

func BenchmarkTokenizeParallel_1KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 1024, true, "tiny") // 1KB input
}

func BenchmarkTokenizeParallel_0_001KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 1, true, "tiny") // 0.001KB input
}

func BenchmarkTokenizeParallel_0_01KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 10, true, "tiny") // 0.01KB input
}

func BenchmarkTokenizeParallel_4KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 4096, true, "tiny") // 4KB input
}

func BenchmarkTokenizeParallel_16KB_tiny(b *testing.B) {
	benchmarkTokenize(b, 16384, true, "tiny") // 16KB input
}

func BenchmarkTokenizeSequential_4KB_phi(b *testing.B) {
	benchmarkTokenize(b, 4096, false, "phi-3") // 1KB input
}

func BenchmarkTokenizeSequential_1KB_phi(b *testing.B) {
	benchmarkTokenize(b, 1024, false, "phi-3") // 1KB input
}

func BenchmarkTokenizeSequential_0_01KB_phi(b *testing.B) {
	benchmarkTokenize(b, 10, false, "phi-3") // 0.01KB input
}

func BenchmarkTokenizeSequential_0_001KB_phi(b *testing.B) {
	benchmarkTokenize(b, 1, false, "phi-3") // 0.001KB input
}

func BenchmarkTokenizeParallel_1KB_phi(b *testing.B) {
	benchmarkTokenize(b, 1024, true, "phi-3") // 1KB input
}

func BenchmarkTokenizeParallel_0_001KB_phi(b *testing.B) {
	benchmarkTokenize(b, 1, true, "phi-3") // 0.001KB input
}

func BenchmarkTokenizeParallel_0_01KB_phi(b *testing.B) {
	benchmarkTokenize(b, 10, true, "phi-3") // 0.01KB input
}

func BenchmarkTokenizeParallel_4KB_phi(b *testing.B) {
	benchmarkTokenize(b, 4096, true, "phi-3") // 4KB input
}

func BenchmarkTokenizeParallel_16KB_phi(b *testing.B) {
	benchmarkTokenize(b, 16384, true, "phi-3") // 16KB input
}
