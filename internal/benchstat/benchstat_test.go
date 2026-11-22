package benchstat

import (
	"math"
	"math/rand/v2"
	"testing"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9g, want %.9g (±%g)", name, got, want, tol)
	}
}

func TestTailAlpha(t *testing.T) {
	near(t, "TailAlpha(5)", TailAlpha(5), 2.866515719e-7, 1e-15)
	near(t, "TailAlpha(1.959964)", TailAlpha(1.959964), 0.025, 1e-7)
}

func TestTQuantile(t *testing.T) {
	// Reference values from standard t tables.
	near(t, "t(0.025, 10)", TQuantile(0.025, 10), 2.228138852, 1e-6)
	near(t, "t(0.005, 1)", TQuantile(0.005, 1), 63.65674116, 1e-4)
	near(t, "t(0.05, 30)", TQuantile(0.05, 30), 1.697260887, 1e-6)
	// With many degrees of freedom t approaches the normal: 5 sigma.
	near(t, "t(5σ, 1e7)", TQuantile(TailAlpha(5), 1e7), 5.0, 1e-3)
}

func TestBetaInc(t *testing.T) {
	if betaInc(2, 3, 0) != 0 || betaInc(2, 3, 1) != 1 {
		t.Error("betaInc bounds")
	}
	// I_x(1, 1) = x; both branches of the continued fraction.
	near(t, "I_0.2(1,1)", betaInc(1, 1, 0.2), 0.2, 1e-12)
	near(t, "I_0.9(1,1)", betaInc(1, 1, 0.9), 0.9, 1e-12)
	// A zero denominator is clamped rather than divided by.
	if v := betaCF(1, 1, 1); math.IsNaN(v) || math.IsInf(v, 0) {
		t.Errorf("betaCF(1, 1, 1) = %v", v)
	}
}

// pairs makes n paired timings with b ~ 1 and a = factor * b, each with
// multiplicative noise of the given relative size.
func pairs(n int, factor, noise float64) (a, b []float64) {
	r := rand.New(rand.NewPCG(1, 2))
	for range n {
		base := 1 + 0.1*r.Float64()
		a = append(a, factor*base*math.Exp(noise*r.NormFloat64()))
		b = append(b, base*math.Exp(noise*r.NormFloat64()))
	}
	return a, b
}

func TestEquivalenceVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		n      int
		factor float64
		noise  float64
		want   Verdict
	}{
		{"same", 2000, 1.0, 0.01, Equivalent},
		{"slower", 2000, 1.05, 0.01, Slower},
		{"faster", 2000, 0.95, 0.01, Faster},
		{"noisy", 20, 1.0, 0.2, Inconclusive},
	} {
		a, b := pairs(tc.n, tc.factor, tc.noise)
		r, err := Equivalence(a, b, 0.01, 5)
		if err != nil {
			t.Fatal(err)
		}
		if r.Verdict != tc.want {
			t.Errorf("%s: verdict %s, want %s (%+v)", tc.name, r.Verdict, tc.want, r)
		}
		if !(r.Low <= r.Ratio && r.Ratio <= r.High) || r.Pairs != tc.n {
			t.Errorf("%s: bad interval %+v", tc.name, r)
		}
	}
}

func TestEquivalenceErrors(t *testing.T) {
	ten := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	bad := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 0}
	for _, tc := range []struct {
		name          string
		a, b          []float64
		margin, sigma float64
	}{
		{"length", ten, ten[:9], 0.01, 5},
		{"few", ten[:9], ten[:9], 0.01, 5},
		{"margin", ten, ten, 0, 5},
		{"sigma", ten, ten, 0.01, math.NaN()},
		{"zero", ten, bad, 0.01, 5},
		{"zero a", bad, ten, 0.01, 5},
	} {
		if _, err := Equivalence(tc.a, tc.b, tc.margin, tc.sigma); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
}
