package note

import (
	"testing"
)

func TestParseQuery_SingleWord(t *testing.T) {
	tokens := ParseQuery("foo")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Text != "foo" || tokens[0].Tag || tokens[0].Exact {
		t.Errorf("unexpected token: %+v", tokens[0])
	}
}

func TestParseQuery_MultipleWords(t *testing.T) {
	tokens := ParseQuery("foo bar baz")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	for _, tok := range tokens {
		if tok.Tag || tok.Exact {
			t.Errorf("expected plain word token, got %+v", tok)
		}
	}
}

func TestParseQuery_QuotedPhrase(t *testing.T) {
	tokens := ParseQuery(`"foo bar"`)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Text != "foo bar" || !tokens[0].Exact {
		t.Errorf("unexpected token: %+v", tokens[0])
	}
}

func TestParseQuery_TagFilter(t *testing.T) {
	tokens := ParseQuery("#daily")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Text != "daily" || !tokens[0].Tag {
		t.Errorf("unexpected token: %+v", tokens[0])
	}
}

func TestParseQuery_Mixed(t *testing.T) {
	tokens := ParseQuery(`meeting #work "action items"`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0].Text != "meeting" || tokens[0].Tag || tokens[0].Exact {
		t.Errorf("token 0: %+v", tokens[0])
	}
	if tokens[1].Text != "work" || !tokens[1].Tag {
		t.Errorf("token 1: %+v", tokens[1])
	}
	if tokens[2].Text != "action items" || !tokens[2].Exact {
		t.Errorf("token 2: %+v", tokens[2])
	}
}

func TestParseQuery_Empty(t *testing.T) {
	tokens := ParseQuery("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestParseQuery_CaseNormalization(t *testing.T) {
	tokens := ParseQuery(`FOO #Daily "Mixed Case"`)
	if tokens[0].Text != "foo" {
		t.Errorf("expected lowercase, got %q", tokens[0].Text)
	}
	if tokens[1].Text != "daily" {
		t.Errorf("expected lowercase tag, got %q", tokens[1].Text)
	}
	if tokens[2].Text != "mixed case" {
		t.Errorf("expected lowercase phrase, got %q", tokens[2].Text)
	}
}

func TestParseQuery_UnclosedQuote(t *testing.T) {
	tokens := ParseQuery(`"foo bar`)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Text != "foo bar" || !tokens[0].Exact {
		t.Errorf("unexpected token: %+v", tokens[0])
	}
}

func TestTextTokens(t *testing.T) {
	tokens := ParseQuery(`meeting #work "action items"`)
	text := TextTokens(tokens)
	if len(text) != 2 {
		t.Fatalf("expected 2 text tokens, got %d", len(text))
	}
}

func TestTagTokens(t *testing.T) {
	tokens := ParseQuery(`meeting #work #daily`)
	tags := TagTokens(tokens)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tag tokens, got %d", len(tags))
	}
}
