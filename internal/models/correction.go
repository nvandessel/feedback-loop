package models

import (
	"time"
)

// Correction represents a captured correction event from a conversation
type Correction struct {
	// Unique identifier (content-addressed hash)
	ID string `json:"id" yaml:"id"`

	// When this correction occurred
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// The context when the correction happened
	Context ContextSnapshot `json:"context" yaml:"context"`

	// What the agent did (that was wrong)
	AgentAction string `json:"agent_action" yaml:"agent_action"`

	// What the human said in response
	HumanResponse string `json:"human_response" yaml:"human_response"`

	// What the agent should have done (extracted/inferred)
	CorrectedAction string `json:"corrected_action" yaml:"corrected_action"`

	// Conversation reference
	ConversationID string `json:"conversation_id" yaml:"conversation_id"`
	TurnNumber     int    `json:"turn_number" yaml:"turn_number"`

	// Who made the correction
	Corrector string `json:"corrector" yaml:"corrector"`

	// Extra tags provided by the user (merged with inferred tags during extraction)
	ExtraTags []string `json:"extra_tags,omitempty" yaml:"extra_tags,omitempty"`

	// Processing state
	Processed   bool       `json:"processed" yaml:"processed"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" yaml:"processed_at,omitempty"`
}
