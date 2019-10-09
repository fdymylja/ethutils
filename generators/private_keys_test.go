package generators

import "testing"

func TestPrivateKeys(t *testing.T) {
	amount := 10
	keys, err := PrivateKeys(amount)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k == nil {
			t.Fatal(err)
		}
	}
}
