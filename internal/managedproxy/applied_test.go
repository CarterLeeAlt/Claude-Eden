package managedproxy

import "testing"

func TestEndpointAloneCannotAuthorizeNetworking(t *testing.T) {
	profile := t.TempDir()
	p, _ := Load(profile)
	if _, err := Applied(profile, "socks5://127.0.0.1:17896", ""); err == nil {
		t.Fatal("old endpoint accepted without nodes")
	}
	if err := p.SetNode(Node{ID: Token()[:24], Name: "qa", Type: "socks5", Server: "127.0.0.1", Port: 1080}); err != nil {
		t.Fatal(err)
	}
	p.Revision = "applied"
	p.Selected = p.Nodes[0].ID
	if err := p.Save(profile); err != nil {
		t.Fatal(err)
	}
	for _, input := range [][2]string{{p.Endpoint(), ""}, {p.Endpoint(), "old"}, {"socks5://127.0.0.1:7890", p.Revision}} {
		if _, err := Applied(profile, input[0], input[1]); err == nil {
			t.Fatal("unapplied endpoint accepted")
		}
	}
	if _, err := Applied(profile, p.Endpoint(), p.Revision); err != nil {
		t.Fatal(err)
	}
	p.Selected = ""
	_ = p.Save(profile)
	if _, err := Applied(profile, p.Endpoint(), p.Revision); err == nil {
		t.Fatal("removed node retained networking authorization")
	}
}
