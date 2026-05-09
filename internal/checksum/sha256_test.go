package checksum

import "testing"

func TestOf(t *testing.T) {
	t.Parallel()
	got := Of([]byte("hello"))
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("Of: got %s want %s", got, want)
	}
}
