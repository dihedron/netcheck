package checks

import "testing"

// The mock bundles count which fields a template reads.
func TestMockBundlesTrackAccess(t *testing.T) {
	b := MockBundles[0]
	if b.IDAccessed() {
		t.Fatal("nothing has been read yet")
	}
	_ = b.ID()
	_ = b.Description()
	_ = b.Timeout()
	_ = b.Retries()
	_ = b.Wait()
	_ = b.Concurrency()
	checks := b.Checks()
	for name, accessed := range map[string]bool{
		"ID": b.IDAccessed(), "Description": b.DescriptionAccessed(), "Timeout": b.TimeoutAccessed(),
		"Retries": b.RetriesAccessed(), "Wait": b.WaitAccessed(), "Concurrency": b.ConcurrencyAccessed(),
		"Checks": b.ChecksAccessed(),
	} {
		if !accessed {
			t.Errorf("%s not tracked", name)
		}
	}
	c := checks[0]
	_ = c.Description()
	_ = c.Timeout()
	_ = c.Retries()
	_ = c.Wait()
	_ = c.Address()
	_ = c.Protocol()
	_ = c.Result()
	for name, accessed := range map[string]bool{
		"Description": c.DescriptionAccessed(), "Timeout": c.TimeoutAccessed(), "Retries": c.RetriesAccessed(),
		"Wait": c.WaitAccessed(), "Address": c.AddressAccessed(), "Protocol": c.ProtocolAccessed(), "Result": c.ResultAccessed(),
	} {
		if !accessed {
			t.Errorf("check %s not tracked", name)
		}
	}
}
