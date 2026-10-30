package operationshttp

// restrictRequest is the payload of POST /ops/users/:id/restrict.
type restrictRequest struct {
	Type   string `json:"type" validate:"required,max=40"`
	Reason string `json:"reason" validate:"required,max=200"`
	Scope  string `json:"scope" validate:"required,max=120"`
}

// unrestrictRequest is the payload of POST /ops/users/:id/unrestrict.
type unrestrictRequest struct {
	RestrictionID string `json:"restriction_id" validate:"required,max=64"`
	Reason        string `json:"reason" validate:"required,max=200"`
}

// decideAuditRequest is the payload of the audit decision command. Version is
// the optimistic lock value returned by the claim command.
type decideAuditRequest struct {
	Action  string `json:"action" validate:"required,oneof=approve reject"`
	Reason  string `json:"reason" validate:"max=200"`
	Version int    `json:"version" validate:"required,min=1"`
}

// orderCommandRequest is the payload of POST /ops/orders/:order_no/commands.
// Basis is the ticket reference that justifies the人工动作; without it the
// console refuses to touch a live order.
type orderCommandRequest struct {
	Command string `json:"command" validate:"required,oneof=reassign waive_cancel_fee safe_end review_pricing"`
	Basis   string `json:"basis" validate:"required,min=5,max=200"`
}

// ticketCommandRequest is the payload of POST /ops/tickets/:ticket_no/commands.
// Message carries what was said or why: no ticket state may change without it.
type ticketCommandRequest struct {
	Command string `json:"command" validate:"required,oneof=claim reply wait_user resume resolve close transfer escalate revoke merge"`
	Message string `json:"message" validate:"required,min=5,max=200"`
	// Assignee is the transfer target; MergeNo is the duplicate to fold in.
	Assignee string `json:"assignee" validate:"omitempty,min=3,max=64"`
	MergeNo  string `json:"merge_no" validate:"omitempty,min=1,max=64"`
}

// ticketAppealRequest is the payload of the appeal route: a passenger unhappy
// with the result opens a new round handled by another, senior agent.
type ticketAppealRequest struct {
	Message string `json:"message" validate:"required,min=5,max=200"`
}
