package dns

import (
	"context"
	"net"
	"testing"

	dns "golang.org/x/net/dns/dnsmessage"
	"tailscale.com/health"
	"tailscale.com/net/netmon"
	"tailscale.com/net/tsdial"
	"tailscale.com/util/dnsname"
)

func TestQuad100Conn(t *testing.T) {
	f := fakeOSConfigurator{
		SplitDNS: true,
		BaseConfig: OSConfig{
			Nameservers:   mustIPs("8.8.8.8"),
			SearchDomains: fqdns("coffee.shop"),
		},
	}
	m := NewManager(t.Logf, &f, new(health.Tracker), tsdial.NewDialer(netmon.NewStatic()), nil, nil, "")
	m.resolver.TestOnlySetHook(f.SetResolver)
	m.Set(Config{
		Hosts: hosts(
			"dave.ts.net.", "1.2.3.4",
			"matt.ts.net.", "2.3.4.5"),
		Routes:        upstreams("ts.net", ""),
		SearchDomains: fqdns("tailscale.com", "universe.tf"),
	})
	defer m.Down()

	q100 := &Quad100conn{
		Ctx:        context.Background(),
		DnsManager: m,
	}
	defer q100.Close()

	var b []byte
	domain := dnsname.FQDN("matt.ts.net.")

	// Send a query
	b = mkDNSRequest(domain, dns.TypeA, addEDNS)
	_, err := q100.Write(b)
	if err != nil {
		t.Fatal(err)
	}

	resp := make([]byte, 100)
	if _, err := q100.Read(resp); err != nil {
		t.Fatalf("reading data: %v", err)
	}

	var parser dns.Parser
	if _, err := parser.Start(resp); err != nil {
		t.Errorf("parser.Start() failed: %v", err)
	}
	_, err = parser.Question()
	if err != nil {
		t.Errorf("parser.Question(): %v", err)
	}
	if err := parser.SkipAllQuestions(); err != nil {
		t.Errorf("parser.SkipAllQuestions(): %v", err)
	}
	ah, err := parser.AnswerHeader()
	if err != nil {
		t.Errorf("parser.AnswerHeader(): %v", err)
	}
	if ah.Type != dns.TypeA {
		t.Errorf("unexpected answer type: got %v, want %v", ah.Type, dns.TypeA)
	}
	res, err := parser.AResource()
	if err != nil {
		t.Errorf("parser.AResource(): %v", err)
	}
	if net.IP(res.A[:]).String() != "2.3.4.5" {
		t.Fatalf("dns query did not return expected result")
	}

}

func TestQuad100Resolver(t *testing.T) {
	f := fakeOSConfigurator{
		SplitDNS: true,
		BaseConfig: OSConfig{
			Nameservers:   mustIPs("8.8.8.8"),
			SearchDomains: fqdns("coffee.shop"),
		},
	}
	m := NewManager(t.Logf, &f, new(health.Tracker), tsdial.NewDialer(netmon.NewStatic()), nil, nil, "")
	m.resolver.TestOnlySetHook(f.SetResolver)
	m.Set(Config{
		Hosts: hosts(
			"dave.ts.net.", "1.2.3.4",
			"matt.ts.net.", "2.3.4.5"),
		Routes:        upstreams("ts.net", ""),
		SearchDomains: fqdns("tailscale.com", "universe.tf"),
	})
	defer m.Down()

	resolver := Quad100Resolver(context.Background(), m)

	ip, err := resolver("matt.ts.net")
	if err != nil {
		t.Errorf("could not resolve host: %v", err)
	}

	if ip.String() != "2.3.4.5" {
		t.Fatalf("dns query did not return expected result")
	}

}
