// Author: Sunil Jain, 2026-10-25
// Brownfield authority decisions are pure and fail closed; no legacy system is mutated.
package brownfield

import (
	"errors"
	"strings"
)

type Mode string

const (
	Observe   Mode = "OBSERVE"
	Advise    Mode = "ADVISE"
	Delegated Mode = "DELEGATED"
	Authority Mode = "AUTHORITY"
)

type Scope struct {
	TenantID string `json:"tenant_id"`
	PlantID  string `json:"plant_id"`
	Domain   string `json:"domain"`
	ObjectID string `json:"object_id"`
}

type Grant struct {
	Scope      Scope  `json:"scope"`
	Mode       Mode   `json:"mode"`
	WriterID   string `json:"writer_id,omitempty"`
	ApprovedBy string `json:"approved_by,omitempty"`
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// CanCommit decides if a business-state write is authorized. Its result is not
// a distributed lock, lease, or a substitute for database concurrency control.
func CanCommit(grant Grant, requested Scope, writerID string) Decision {
	if strings.TrimSpace(requested.TenantID) == "" || strings.TrimSpace(requested.PlantID) == "" || strings.TrimSpace(requested.Domain) == "" || strings.TrimSpace(requested.ObjectID) == "" {
		return Decision{Reason: "Missing factory or work reference"}
	}
	if grant.Scope != requested {
		return Decision{Reason: "This work is outside the approved area"}
	}
	switch grant.Mode {
	case Observe, Advise:
		return Decision{Reason: "Agiparel can watch and suggest, not change factory records"}
	case Delegated, Authority:
		if strings.TrimSpace(grant.ApprovedBy) == "" {
			return Decision{Reason: "Manager approval is missing"}
		}
		if strings.TrimSpace(writerID) == "" || writerID != grant.WriterID {
			return Decision{Reason: "Another system owns this record"}
		}
		return Decision{Allowed: true, Reason: "Approved"}
	default:
		return Decision{Reason: "Unknown control mode"}
	}
}

func ValidateGrant(g Grant) error {
	if strings.TrimSpace(g.Scope.TenantID) == "" || strings.TrimSpace(g.Scope.PlantID) == "" || strings.TrimSpace(g.Scope.Domain) == "" || strings.TrimSpace(g.Scope.ObjectID) == "" {
		return errors.New("grant requires tenant, plant, domain and object")
	}
	switch g.Mode {
	case Observe, Advise:
		if g.WriterID != "" {
			return errors.New("read-only modes cannot declare an Agiparel writer")
		}
	case Delegated, Authority:
		if strings.TrimSpace(g.WriterID) == "" || strings.TrimSpace(g.ApprovedBy) == "" {
			return errors.New("write authority requires writer and approval")
		}
	default:
		return errors.New("unknown authority mode")
	}
	return nil
}
