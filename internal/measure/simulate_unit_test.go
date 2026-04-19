package measure

import (
	"math/rand"
	"testing"
)

func TestAddNoise_ZeroScale(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	got := addNoise(10.0, 0, "lognormal", rng)
	if got != 10.0 {
		t.Errorf("expected 10.0 with zero scale, got %f", got)
	}
}

func TestAddNoise_Lognormal(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	got := addNoise(10.0, 0.1, "lognormal", rng)
	if got <= 0 {
		t.Errorf("lognormal noise should produce positive value, got %f", got)
	}
	if got == 10.0 {
		t.Error("expected noise to change the value")
	}
}

func TestAddNoise_Gaussian(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	got := addNoise(10.0, 0.1, "gaussian", rng)
	if got < 0 {
		t.Errorf("gaussian noise with clipping should be >= 0, got %f", got)
	}
}

func TestAddNoise_NegativeScale(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	got := addNoise(10.0, -1, "lognormal", rng)
	if got != 10.0 {
		t.Errorf("negative scale should return original value, got %f", got)
	}
}
