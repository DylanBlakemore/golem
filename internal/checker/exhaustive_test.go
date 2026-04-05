package checker

import (
	"testing"
)

// --- Exhaustive sum types ---

func TestExhaustiveMatchAllVariants(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }
  | Triangle { base: Float, height: Float }

fn area(s: Shape): Float do
  match s do
    | Circle { radius } -> radius * radius
    | Rectangle { width, height } -> width * height
    | Triangle { base, height } -> base * height
  end
end`)
	expectNoErrors(t, errs)
}

func TestExhaustiveMatchUnitVariants(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn name(c: Color): String do
  match c do
    | Red -> "red"
    | Green -> "green"
    | Blue -> "blue"
  end
end`)
	expectNoErrors(t, errs)
}

// --- Non-exhaustive sum types ---

func TestNonExhaustiveMissingVariant(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }
  | Triangle { base: Float, height: Float }

fn area(s: Shape): Float do
  match s do
    | Circle { radius } -> radius * radius
    | Rectangle { width, height } -> width * height
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
	expectErrorContains(t, errs, "Triangle")
}

func TestNonExhaustiveMissingMultipleVariants(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn name(c: Color): String do
  match c do
    | Red -> "red"
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
	expectErrorContains(t, errs, "Green")
	expectErrorContains(t, errs, "Blue")
}

// --- Wildcard catch-all ---

func TestWildcardSatisfiesExhaustiveness(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn isRed(c: Color): String do
  match c do
    | Red -> "yes"
    | _ -> "no"
  end
end`)
	expectNoErrors(t, errs)
}

func TestVarPatternSatisfiesExhaustiveness(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn show(c: Color): String do
  match c do
    | Red -> "red"
    | other -> "not red"
  end
end`)
	expectNoErrors(t, errs)
}

// --- Redundant arm detection ---

func TestRedundantArmAfterWildcard(t *testing.T) {
	info, _ := check(`type Color =
  | Red
  | Green
  | Blue

fn name(c: Color): String do
  match c do
    | Red -> "red"
    | _ -> "other"
    | Green -> "green"
  end
end`)
	if len(info.Warnings) == 0 {
		t.Fatal("expected warning for redundant arm")
	}
	found := false
	for _, w := range info.Warnings {
		if containsStr(w.Message, "unreachable") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'unreachable' warning")
		for _, w := range info.Warnings {
			t.Logf("  warning: %s", w.Message)
		}
	}
}

func TestRedundantArmDuplicateConstructor(t *testing.T) {
	info, _ := check(`type Color =
  | Red
  | Green
  | Blue

fn name(c: Color): String do
  match c do
    | Red -> "red"
    | Green -> "green"
    | Red -> "also red"
    | Blue -> "blue"
  end
end`)
	if len(info.Warnings) == 0 {
		t.Fatal("expected warning for redundant arm")
	}
	found := false
	for _, w := range info.Warnings {
		if containsStr(w.Message, "unreachable") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'unreachable' warning")
	}
}

// --- Bool exhaustiveness ---

func TestBoolExhaustiveTrueAndFalse(t *testing.T) {
	_, errs := check(`fn describe(b: Bool): String do
  match b do
    | true -> "yes"
    | false -> "no"
  end
end`)
	expectNoErrors(t, errs)
}

func TestBoolNonExhaustiveMissingFalse(t *testing.T) {
	_, errs := check(`fn describe(b: Bool): String do
  match b do
    | true -> "yes"
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
	expectErrorContains(t, errs, "false")
}

func TestBoolWildcardExhaustive(t *testing.T) {
	_, errs := check(`fn describe(b: Bool): String do
  match b do
    | true -> "yes"
    | _ -> "no"
  end
end`)
	expectNoErrors(t, errs)
}

// --- Infinite types require wildcard ---

func TestIntMatchRequiresWildcard(t *testing.T) {
	_, errs := check(`fn describe(n: Int): String do
  match n do
    | 1 -> "one"
    | 2 -> "two"
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
}

func TestIntMatchWithWildcard(t *testing.T) {
	_, errs := check(`fn describe(n: Int): String do
  match n do
    | 1 -> "one"
    | 2 -> "two"
    | _ -> "other"
  end
end`)
	expectNoErrors(t, errs)
}

func TestStringMatchRequiresWildcard(t *testing.T) {
	_, errs := check(`fn describe(s: String): String do
  match s do
    | "hello" -> "greeting"
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
}

func TestStringMatchWithWildcard(t *testing.T) {
	_, errs := check(`fn describe(s: String): String do
  match s do
    | "hello" -> "greeting"
    | _ -> "unknown"
  end
end`)
	expectNoErrors(t, errs)
}

// --- Edge cases ---

func TestSingleVariantExhaustive(t *testing.T) {
	_, errs := check(`type Wrapper =
  | Val { inner: Int }

fn unwrap(w: Wrapper): Int do
  match w do
    | Val { inner } -> inner
  end
end`)
	expectNoErrors(t, errs)
}

func TestEmptyMatchNonExhaustive(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green

fn name(c: Color): String do
  match c do
  end
end`)
	expectErrorContains(t, errs, "non-exhaustive")
}

func TestAllWildcardExhaustive(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green

fn name(c: Color): String do
  match c do
    | _ -> "color"
  end
end`)
	expectNoErrors(t, errs)
}
