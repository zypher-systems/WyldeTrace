package kea

import (
	"context"
	"testing"
)

func TestDryRunSync(t *testing.T) {
	c := &Client{URL: "http://127.0.0.1:8000", DryRun: true}
	res, err := c.SyncPools(context.Background(),
		[]DHCPRange{{SubnetCIDR: "10.0.0.0/24", PoolStart: "10.0.0.50", PoolEnd: "10.0.0.100"}},
		[]Reservation{{IPAddress: "10.0.0.10", HWAddress: "aa:bb:cc:dd:ee:ff", Hostname: "host1"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.StatusCode != 200 {
		t.Fatalf("%+v", res)
	}
}
