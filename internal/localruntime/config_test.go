package localruntime

import "testing"

func TestCompanionContractDoesNotDependOnDeploymentEnvironment(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("SECTILE_JIRA_OAUTH_REDIRECT_URL", "https://other.example/callback")
	info := Info()
	if info.Port != 52963 || info.Origin != "http://127.0.0.1:52963" || info.OAuthCallbacks["jira"] != "http://127.0.0.1:52963/auth/jira/callback" {
		t.Fatalf("unexpected contract: %+v", info)
	}
	t.Setenv("SECTILE_LOCAL_RUNTIME_KEY", "")
	if Callback("jira") != "" {
		t.Fatal("web deployment acquired a companion callback")
	}
	t.Setenv("SECTILE_LOCAL_RUNTIME_KEY", "private")
	if Callback("jira") != info.OAuthCallbacks["jira"] {
		t.Fatal("companion callback missing")
	}
	for _, tc := range [][3]string{{"127.0.0.1", "0", "data"}, {"0.0.0.0", "52963", "data"}, {"127.0.0.1", "52964", "data"}, {"127.0.0.1", "52963", ""}} {
		if Validate(tc[0], tc[1], tc[2]) == nil {
			t.Fatalf("accepted incompatible runtime: %v", tc)
		}
	}
	if err := Validate("127.0.0.1", "52963", "data"); err != nil {
		t.Fatal(err)
	}
}
