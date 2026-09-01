package frontend

import "testing"

func TestDistContainsIndex(t *testing.T) {
	f, err := Dist.Open("dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}
