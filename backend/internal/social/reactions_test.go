package social

import "testing"

func TestNormalizeReactionTypeDefaultsToLike(t *testing.T) {
	t.Parallel()

	reactionType, err := normalizeReactionType("  ")
	if err != nil {
		t.Fatalf("normalizeReactionType returned error: %v", err)
	}

	if reactionType != reactionLike {
		t.Fatalf("expected reaction %q, got %q", reactionLike, reactionType)
	}
}

func TestNormalizeReactionTypeRejectsUnsupportedReaction(t *testing.T) {
	t.Parallel()

	_, err := normalizeReactionType("love")
	if err == nil {
		t.Fatal("expected unsupported reaction to return validation error")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Fields["reaction"] != "Reaction type is invalid." {
		t.Fatalf("unexpected reaction error: %q", validationErr.Fields["reaction"])
	}
}
