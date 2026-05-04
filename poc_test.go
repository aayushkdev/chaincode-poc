package chaincodepoc_test

import (
	"encoding/json"
	"testing"

	"github.com/aayush/chaincode-poc/runner"
	"github.com/aayush/chaincode-poc/samplecc"
	"github.com/aayush/chaincode-poc/shimadapter"
)

func TestAssetLifecycleThroughAdapter(t *testing.T) {
	r := newRunner(t)

	_, err := r.Invoke("CreateAsset", "asset7", "blue", "5", "alice", "100")
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	readResp, err := r.Query("ReadAsset", "asset7")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}

	var asset samplecc.Asset
	if err := json.Unmarshal(readResp.Response.Payload, &asset); err != nil {
		t.Fatalf("unmarshal asset: %v", err)
	}
	if asset.Owner != "alice" || asset.Color != "blue" || asset.AppraisedValue != 100 || asset.Size != 5 {
		t.Fatalf("unexpected asset: %+v", asset)
	}

	oldOwnerResp, err := r.Invoke("TransferAsset", "asset7", "bob")
	if err != nil {
		t.Fatalf("transfer asset: %v", err)
	}
	if string(oldOwnerResp.Response.Payload) != "alice" {
		t.Fatalf("expected old owner alice, got %q", string(oldOwnerResp.Response.Payload))
	}

	readResp, err = r.Query("ReadAsset", "asset7")
	if err != nil {
		t.Fatalf("read updated asset: %v", err)
	}
	if err := json.Unmarshal(readResp.Response.Payload, &asset); err != nil {
		t.Fatalf("unmarshal updated asset: %v", err)
	}
	if asset.Owner != "bob" {
		t.Fatalf("expected owner bob, got %q", asset.Owner)
	}

	if _, err := r.Invoke("DeleteAsset", "asset7"); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if _, err := r.Query("ReadAsset", "asset7"); err == nil {
		t.Fatalf("expected missing asset error after delete")
	}
}

func TestRangeQueryUsesCommittedState(t *testing.T) {
	r := newRunner(t)

	if _, err := r.Invoke("InitLedger"); err != nil {
		t.Fatalf("init ledger: %v", err)
	}

	resp, err := r.Query("GetAllAssets")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}

	var assets []samplecc.Asset
	if err := json.Unmarshal(resp.Response.Payload, &assets); err != nil {
		t.Fatalf("unmarshal asset list: %v", err)
	}
	if len(assets) != 6 {
		t.Fatalf("expected 6 assets, got %d", len(assets))
	}
	if assets[0].ID != "asset1" || assets[1].ID != "asset2" || assets[5].ID != "asset6" {
		t.Fatalf("unexpected asset order: %+v", assets)
	}
}

func TestUpdateAssetThroughAdapter(t *testing.T) {
	r := newRunner(t)

	if _, err := r.Invoke("CreateAsset", "asset9", "black", "7", "maya", "900"); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := r.Invoke("UpdateAsset", "asset9", "white", "11", "maya", "950"); err != nil {
		t.Fatalf("update asset: %v", err)
	}

	resp, err := r.Query("ReadAsset", "asset9")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}

	var asset samplecc.Asset
	if err := json.Unmarshal(resp.Response.Payload, &asset); err != nil {
		t.Fatalf("unmarshal asset: %v", err)
	}
	if asset.Color != "white" || asset.Size != 11 || asset.AppraisedValue != 950 {
		t.Fatalf("unexpected updated asset: %+v", asset)
	}
}

func newRunner(t *testing.T) *runner.Runner {
	t.Helper()

	cc, err := samplecc.NewChaincode()
	if err != nil {
		t.Fatalf("create chaincode: %v", err)
	}

	return runner.New(cc, shimadapter.NewInMemoryStateClient(), "assets")
}
