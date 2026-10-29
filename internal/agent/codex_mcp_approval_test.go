package agent

import (
	"encoding/json"
	"testing"
)

func TestCodexMCPFormRequiresExplicitTypedAnswers(t *testing.T) {
	frame := codexFrame{ID: json.RawMessage(`7`), Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"serverName":"sectile_companion","mode":"form","message":"Approve this operation?","requestedSchema":{"type":"object","properties":{"approved":{"type":"boolean"},"scope":{"type":"string","enum":["once","session"]}}}}`)}
	a, ok := codexApproval(frame)
	if !ok || a.Tool != askUserQuestion {
		t.Fatal("MCP form was not presented as questions")
	}
	answers := map[string]string{}
	for q, key := range a.QuestionIDs {
		if key == "approved" {
			answers[q] = "true"
		} else {
			answers[q] = "once"
		}
	}
	result, err := codexMCPAnswer(a, "answer", answers)
	if err != nil {
		t.Fatal(err)
	}
	content := result["content"].(map[string]any)
	if content["approved"] != true || content["scope"] != "once" || result["action"] != "accept" {
		t.Fatal("MCP form lost its field types")
	}
	for q, key := range a.QuestionIDs {
		if key == "approved" {
			answers[q] = "yes"
		}
	}
	if _, err := codexMCPAnswer(a, "answer", answers); err == nil {
		t.Fatal("unrecognized boolean accepted")
	}
	denied, err := codexMCPAnswer(a, "deny", nil)
	if err != nil || denied["action"] != "decline" || denied["content"] != nil {
		t.Fatal("skip must decline without submitting answers")
	}
}

func TestCodexMCPConsentNeverOffersPermanentApproval(t *testing.T) {
	a, ok := codexApproval(codexFrame{ID: json.RawMessage(`8`), Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"serverName":"sectile_companion","mode":"form","message":"Allow this call?","requestedSchema":{"type":"object","properties":{}}}`)})
	if !ok || len(a.Choices) != 2 {
		t.Fatal("missing explicit consent")
	}
	if _, err := codexMCPAnswer(a, "always", nil); err == nil {
		t.Fatal("unoffered permanent consent accepted")
	}
}
