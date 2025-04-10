package libollama_test

// this test will only work if ~/.ollama is writable for the current user
// func TestPrepareTokenizer(t *testing.T) {
// 	t.Run("Prepare llama tokenizer", func(t *testing.T) {
// 		err := libollama.PullModel(context.Background(), "smollm2:135m", func(status, digest string, completed, total int64) {
// 			t.Logf("Status: %s, Total: %d, Completed: %d, Digest: %s", status, total, completed, digest)
// 		})
// 		if err != nil {
// 			t.Fatalf("PrepareTokenizer failed: %v", err)
// 		}
// 	})
// }
