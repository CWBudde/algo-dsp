package delay

import "testing"

func TestNewValidation(t *testing.T) {
	if _, err := New(0); err == nil {
		t.Fatal("expected error for size=0")
	}
}

func TestReadWrite(t *testing.T) {
	d, err := New(8)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		d.Write(float64(i))
	}
	if got := d.Read(1); got != 7 {
		t.Fatalf("got %v want 7", got)
	}
	if got := d.Read(3); got != 5 {
		t.Fatalf("got %v want 5", got)
	}
}

func TestReadFractionalLinearRamp(t *testing.T) {
	d, err := New(16)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < d.Len(); i++ {
		d.Write(float64(i))
	}
	if got := d.ReadFractional(3.5); got < 12.49 || got > 12.51 {
		t.Fatalf("got %v want about 12.5", got)
	}
}
