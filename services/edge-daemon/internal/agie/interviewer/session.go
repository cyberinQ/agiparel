package interviewer

import (
	"errors"
	"sync"
	"time"
)

type PersonaType string

const (
	PersonaGM         PersonaType = "FACTORY_GM"
	PersonaSupervisor PersonaType = "LINE_SUPERVISOR"
	PersonaStoreLead  PersonaType = "STORE_MANAGER"
	PersonaIELead     PersonaType = "INDUSTRIAL_ENGINEER"
)

type QAPair struct {
	Index       int         `json:"index"`
	Timestamp   int64       `json:"timestamp"`
	Persona     PersonaType `json:"persona"`
	Question    string      `json:"question"`
	Answer      string      `json:"answer"`
	Extracted   interface{} `json:"extracted,omitempty"`
}

type InterviewSession struct {
	sync.Mutex
	SessionID    string              `json:"session_id"`
	TenantID     string              `json:"tenant_id"`
	PlantID      string              `json:"plant_id"`
	StartedAt    int64               `json:"started_at"`
	IsComplete   bool                `json:"is_complete"`
	History      []QAPair            `json:"history"`
	CompiledData map[string]interface{} `json:"compiled_data"`
}

func NewInterviewSession(sessionID, tenantID, plantID string) *InterviewSession {
	return &InterviewSession{
		SessionID:    sessionID,
		TenantID:     tenantID,
		PlantID:      plantID,
		StartedAt:    time.Now().UTC().UnixMilli(),
		IsComplete:   false,
		History:      []QAPair{},
		CompiledData: make(map[string]interface{}),
	}
}

func (s *InterviewSession) RecordExchange(persona PersonaType, question, answer string, metadata interface{}) (QAPair, error) {
	s.Lock()
	defer s.Unlock()

	if s.IsComplete {
		return QAPair{}, errors.New("ERR_SESSION_CLOSED: Interview session is already completed and locked")
	}

	idx := len(s.History) + 1
	pair := QAPair{
		Index:     idx,
		Timestamp: time.Now().UTC().UnixMilli(),
		Persona:   persona,
		Question:  question,
		Answer:    answer,
		Extracted: metadata,
	}

	s.History = append(s.History, pair)
	return pair, nil
}

func (s *InterviewSession) FinalizeAndCompile() (map[string]interface{}, error) {
	s.Lock()
	defer s.Unlock()

	if len(s.History) == 0 {
		return nil, errors.New("ERR_EMPTY_INTERVIEW: Cannot compile session with zero Q&A records")
	}

	s.IsComplete = true
	// Aggregate structured parameters for downstream sub-agents (DB-Smith & Logic-Weaver)
	s.CompiledData["total_exchanges"] = len(s.History)
	s.CompiledData["finalized_at"] = time.Now().UTC().UnixMilli()
	
	return s.CompiledData, nil
}