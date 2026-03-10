package dashboard

import (
	"testing"
)

func TestEmbeddedAssetsContainsIndexHTML(t *testing.T) {
	data, err := Assets.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("failed to read embedded index.html: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("embedded index.html is empty")
	}
}
