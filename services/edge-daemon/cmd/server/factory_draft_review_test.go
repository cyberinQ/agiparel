package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func makeDraftRow(kind, key, id string, attrs string) structDraftRow {
	return structDraftRow{Type: kind, Key: key, Source: factoryDraftSource{CandidateID: id, DocumentID: "DOC-0123456789abcdef0123456789abcdef", Filename: "factory.csv", SourceOrdinal: 3, Attributes: json.RawMessage(attrs)}}
}
func TestFactoryDraftCanonicalAndConflicts(t *testing.T) {
	rows := []structDraftRow{
		makeDraftRow("MACHINE", "MC-1", "FC-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{"machine_type":"Lockstitch","machine_id":"MC-1"}`),
		makeDraftRow("MACHINE", "MC-1", "FC-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", `{"machine_id":"MC-1","machine_type":"Lockstitch"}`),
		makeDraftRow("MACHINE", "MC-1", "FC-cccccccccccccccccccccccccccccccc", `{"machine_type":"Overlock","machine_id":"MC-1"}`),
		makeDraftRow("RATE", "Labor:USD", "FC-dddddddddddddddddddddddddddddddd", `{"rate":"7.50","currency":"USD"}`),
	}
	got := summarizeFactoryDrafts(rows, nil)
	if len(got) != 2 || got[0].EntityType != "MACHINE" {
		t.Fatalf("grouping/sort: %+v", got)
	}
	if got[0].EvidenceState != "CONFLICT" || got[0].DistinctVersions != 2 || len(got[0].Sources) != 3 || !got[0].NeedsAttention {
		t.Fatalf("conflict not found: %+v", got[0])
	}
	if got[1].EvidenceState != "SINGLE_SOURCE" {
		t.Fatalf("wrong isolated evidence state: %+v", got[1])
	}
	decisions := map[string]factoryDraftDecision{draftGroupID("MACHINE", "MC-1"): {EntityType: "MACHINE", EntityKey: "MC-1", Decision: "VERIFY_DRAFT", CandidateID: "FC-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Revision: 2, Note: "Identified by onsite inspection"}}
	got = summarizeFactoryDrafts(rows, decisions)
	if got[0].Revision != 2 || got[0].Decision != "VERIFY_DRAFT" || !got[0].NeedsAttention {
		t.Fatal("conflicting evidence may not disappear after review")
	}
}
func TestFactoryDraftSourceAgreement(t *testing.T) {
	rows := []structDraftRow{makeDraftRow("SKILL", "E17:Hem", "FC-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{"level":"2"}`), makeDraftRow("SKILL", "E17:Hem", "FC-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", `{"level":"2"}`)}
	g := summarizeFactoryDrafts(rows, nil)
	if len(g) != 1 || g[0].EvidenceState != "CORROBORATED" {
		t.Fatalf("expected agreement: %+v", g)
	}
}
func TestFactoryDraftReviewRequestValidation(t *testing.T) {
	valid := factoryDraftReviewRequest{EntityType: "RATE", EntityKey: "Labor:USD", CandidateID: "FC-0123456789abcdef0123456789abcdef", Decision: "VERIFY_DRAFT", ExpectedRevision: 0}
	if err := validDraftReview(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []factoryDraftReviewRequest{
		{EntityType: "ORDER", EntityKey: valid.EntityKey, CandidateID: valid.CandidateID, Decision: valid.Decision},
		{EntityType: valid.EntityType, EntityKey: "", CandidateID: valid.CandidateID, Decision: valid.Decision},
		{EntityType: valid.EntityType, EntityKey: " bad", CandidateID: valid.CandidateID, Decision: valid.Decision},
		{EntityType: valid.EntityType, EntityKey: valid.EntityKey, CandidateID: "../bad", Decision: valid.Decision},
		{EntityType: valid.EntityType, EntityKey: valid.EntityKey, CandidateID: valid.CandidateID, Decision: "APPROVE_OPERATIONAL"},
		{EntityType: valid.EntityType, EntityKey: valid.EntityKey, CandidateID: valid.CandidateID, Decision: valid.Decision, ExpectedRevision: -1},
		{EntityType: valid.EntityType, EntityKey: valid.EntityKey, CandidateID: valid.CandidateID, Decision: valid.Decision, Note: strings.Repeat("x", 1001)},
	} {
		if err := validDraftReview(bad); err == nil {
			t.Errorf("unsafe review accepted: %+v", bad)
		}
	}
}
