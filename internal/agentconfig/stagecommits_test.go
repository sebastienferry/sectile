package agentconfig

import (
	"encoding/json"
	"testing"
)

func TestPushStageCommitsContractCompatibility(t *testing.T) {
	for _, raw := range []string{`{}`, `{"pushStageCommits":false}`, `{"pushStageCommits":true}`} {
		var config Config
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			t.Fatal(err)
		}
		want := raw == `{"pushStageCommits":true}`
		if config.PushStageCommits != want {
			t.Fatalf("decoded %s as %v", raw, config.PushStageCommits)
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip Config
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if roundTrip.PushStageCommits != want {
			t.Fatal("contract round trip lost setting")
		}
	}
}
