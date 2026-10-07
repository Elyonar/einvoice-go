package einvoice

import "encoding/json"

// WebhookEventType is a catalogue type, e.g. `invoice.signed`. The catalogue below is as of
// einvoice-js 0.8.0 (the API's `GET /n/v1/webhook-event-types` is authoritative; new types may
// arrive, so any other string is accepted too).
type WebhookEventType string

// The event catalogue.
const (
	WebhookEventInvoiceCreated                           WebhookEventType = "invoice.created"
	WebhookEventInvoiceUpdated                           WebhookEventType = "invoice.updated"
	WebhookEventInvoiceReopened                          WebhookEventType = "invoice.reopened"
	WebhookEventInvoiceDeleted                           WebhookEventType = "invoice.deleted"
	WebhookEventInvoiceFinalised                         WebhookEventType = "invoice.finalised"
	WebhookEventInvoiceSubmitted                         WebhookEventType = "invoice.submitted"
	WebhookEventInvoiceSigned                            WebhookEventType = "invoice.signed"
	WebhookEventInvoiceTransmitted                       WebhookEventType = "invoice.transmitted"
	WebhookEventInvoiceAccepted                          WebhookEventType = "invoice.accepted"
	WebhookEventInvoiceRejected                          WebhookEventType = "invoice.rejected"
	WebhookEventInvoiceParked                            WebhookEventType = "invoice.parked"
	WebhookEventInvoiceRetryScheduled                    WebhookEventType = "invoice.retry_scheduled"
	WebhookEventInvoiceCancellationRequested             WebhookEventType = "invoice.cancellation_requested"
	WebhookEventInvoiceCancellationReverted              WebhookEventType = "invoice.cancellation_reverted"
	WebhookEventInvoiceCancelled                         WebhookEventType = "invoice.cancelled"
	WebhookEventInvoiceSentToBuyer                       WebhookEventType = "invoice.sent_to_buyer"
	WebhookEventInvoicePaymentStatusUpdated              WebhookEventType = "invoice.payment_status.updated"
	WebhookEventInvoicePaymentStatusReportRefused        WebhookEventType = "invoice.payment_status.report_refused"
	WebhookEventInvoiceBuyerDeliveryFailed               WebhookEventType = "invoice.buyer_delivery_failed"
	WebhookEventInvoiceReceived                          WebhookEventType = "invoice.received"
	WebhookEventBuyerCreated                             WebhookEventType = "buyer.created"
	WebhookEventBuyerUpdated                             WebhookEventType = "buyer.updated"
	WebhookEventBuyerDeleted                             WebhookEventType = "buyer.deleted"
	WebhookEventBuyerTaxIDVerified                       WebhookEventType = "buyer.tax_id.verified"
	WebhookEventBuyerTaxIDVerificationFailed             WebhookEventType = "buyer.tax_id.verification_failed"
	WebhookEventBuyerTaxIDVerificationUnavailable        WebhookEventType = "buyer.tax_id.verification_unavailable"
	WebhookEventSellerCreated                            WebhookEventType = "seller.created"
	WebhookEventSellerUpdated                            WebhookEventType = "seller.updated"
	WebhookEventSellerDeleted                            WebhookEventType = "seller.deleted"
	WebhookEventSellerTaxIDVerified                      WebhookEventType = "seller.tax_id.verified"
	WebhookEventSellerTaxIDVerificationFailed            WebhookEventType = "seller.tax_id.verification_failed"
	WebhookEventSellerTaxIDVerificationUnavailable       WebhookEventType = "seller.tax_id.verification_unavailable"
	WebhookEventTaxConnectionActionRequired              WebhookEventType = "tax_connection.action_required"
	WebhookEventTaxConnectionConnected                   WebhookEventType = "tax_connection.connected"
	WebhookEventTaxConnectionSuspended                   WebhookEventType = "tax_connection.suspended"
	WebhookEventTaxConnectionDisconnected                WebhookEventType = "tax_connection.disconnected"
	WebhookEventTaxConnectionKeyInstalled                WebhookEventType = "tax_connection.key_installed"
	WebhookEventTaxConnectionKeyExpiring                 WebhookEventType = "tax_connection.key_expiring"
	WebhookEventTaxConnectionKeyExpired                  WebhookEventType = "tax_connection.key_expired"
	WebhookEventOrganizationUpdated                      WebhookEventType = "organization.updated"
	WebhookEventOrganizationTaxIDVerified                WebhookEventType = "organization.tax_id.verified"
	WebhookEventOrganizationTaxIDVerificationFailed      WebhookEventType = "organization.tax_id.verification_failed"
	WebhookEventOrganizationTaxIDVerificationUnavailable WebhookEventType = "organization.tax_id.verification_unavailable"
	WebhookEventOrganizationPhoneVerified                WebhookEventType = "organization.phone.verified"
	WebhookEventOrganizationLogoChanged                  WebhookEventType = "organization.logo.changed"
	WebhookEventOrganizationOnboardingCompleted          WebhookEventType = "organization.onboarding_completed"
	WebhookEventOrganizationOwnershipTransferred         WebhookEventType = "organization.ownership_transferred"
	WebhookEventOrganizationLiveAccessRequested          WebhookEventType = "organization.live_access.requested"
	WebhookEventOrganizationLiveAccessApproved           WebhookEventType = "organization.live_access.approved"
	WebhookEventOrganizationLiveAccessRejected           WebhookEventType = "organization.live_access.rejected"
	WebhookEventOrganizationLiveAccessRevoked            WebhookEventType = "organization.live_access.revoked"
	WebhookEventOrganizationClosed                       WebhookEventType = "organization.closed"
	WebhookEventOrganizationChildCreated                 WebhookEventType = "organization.child.created"
	WebhookEventMemberJoined                             WebhookEventType = "member.joined"
	WebhookEventMemberRoleChanged                        WebhookEventType = "member.role_changed"
	WebhookEventMemberSuspended                          WebhookEventType = "member.suspended"
	WebhookEventMemberRestored                           WebhookEventType = "member.restored"
	WebhookEventMemberRemoved                            WebhookEventType = "member.removed"
	WebhookEventInvitationCreated                        WebhookEventType = "invitation.created"
	WebhookEventInvitationResent                         WebhookEventType = "invitation.resent"
	WebhookEventInvitationWithdrawn                      WebhookEventType = "invitation.withdrawn"
	WebhookEventInvitationAccepted                       WebhookEventType = "invitation.accepted"
	WebhookEventInvitationExpired                        WebhookEventType = "invitation.expired"
	WebhookEventInvitationLocked                         WebhookEventType = "invitation.locked"
	WebhookEventRoleCreated                              WebhookEventType = "role.created"
	WebhookEventRoleUpdated                              WebhookEventType = "role.updated"
	WebhookEventRoleDeleted                              WebhookEventType = "role.deleted"
	WebhookEventAPIKeyCreated                            WebhookEventType = "api_key.created"
	WebhookEventAPIKeyRotated                            WebhookEventType = "api_key.rotated"
	WebhookEventAPIKeyUpdated                            WebhookEventType = "api_key.updated"
	WebhookEventAPIKeyRevoked                            WebhookEventType = "api_key.revoked"
	WebhookEventAPIKeyExpired                            WebhookEventType = "api_key.expired"
	WebhookEventAPIKeyFlagged                            WebhookEventType = "api_key.flagged"
	WebhookEventAPIKeyExpiring                           WebhookEventType = "api_key.expiring"
	WebhookEventBillingCreditsGranted                    WebhookEventType = "billing.credits.granted"
	WebhookEventBillingCreditsExpired                    WebhookEventType = "billing.credits.expired"
	WebhookEventBillingCreditsAdjusted                   WebhookEventType = "billing.credits.adjusted"
	WebhookEventBillingChargeSettled                     WebhookEventType = "billing.charge.settled"
	WebhookEventBillingChargeRefused                     WebhookEventType = "billing.charge.refused"
	WebhookEventBillingChargeRefunded                    WebhookEventType = "billing.charge.refunded"
	WebhookEventBillingHoldPlaced                        WebhookEventType = "billing.hold.placed"
	WebhookEventBillingHoldCaptured                      WebhookEventType = "billing.hold.captured"
	WebhookEventBillingHoldReleased                      WebhookEventType = "billing.hold.released"
	WebhookEventBillingCreditsTransferred                WebhookEventType = "billing.credits.transferred"
	WebhookEventBillingPoolDrawn                         WebhookEventType = "billing.pool.drawn"
	WebhookEventBillingPoolReturned                      WebhookEventType = "billing.pool.returned"
	WebhookEventBillingPoolPolicyUpdated                 WebhookEventType = "billing.pool_policy.updated"
	WebhookEventBillingBalanceLow                        WebhookEventType = "billing.balance.low"
	WebhookEventBillingBalanceRecovered                  WebhookEventType = "billing.balance.recovered"
	WebhookEventBillingLowBalanceThresholdUpdated        WebhookEventType = "billing.low_balance_threshold.updated"
	WebhookEventBillingDebtRecorded                      WebhookEventType = "billing.debt.recorded"
	WebhookEventBillingDebtSettled                       WebhookEventType = "billing.debt.settled"
	WebhookEventBillingPaymentInitiated                  WebhookEventType = "billing.payment.initiated"
	WebhookEventBillingPaymentSucceeded                  WebhookEventType = "billing.payment.succeeded"
	WebhookEventBillingPaymentFailed                     WebhookEventType = "billing.payment.failed"
	WebhookEventBillingPaymentExpired                    WebhookEventType = "billing.payment.expired"
	WebhookEventBillingPaymentRefunded                   WebhookEventType = "billing.payment.refunded"
	WebhookEventBillingPaymentDisputed                   WebhookEventType = "billing.payment.disputed"
	WebhookEventBillingPaymentDisputeResolved            WebhookEventType = "billing.payment.dispute_resolved"
	WebhookEventBillingSubscriptionFreeAssigned          WebhookEventType = "billing.subscription.free_assigned"
	WebhookEventBillingSubscriptionActivated             WebhookEventType = "billing.subscription.activated"
	WebhookEventBillingSubscriptionRenewed               WebhookEventType = "billing.subscription.renewed"
	WebhookEventBillingSubscriptionRenewalDue            WebhookEventType = "billing.subscription.renewal_due"
	WebhookEventBillingSubscriptionCancelScheduled       WebhookEventType = "billing.subscription.cancel_scheduled"
	WebhookEventBillingSubscriptionResumed               WebhookEventType = "billing.subscription.resumed"
	WebhookEventBillingSubscriptionCancelled             WebhookEventType = "billing.subscription.cancelled"
	WebhookEventBillingSubscriptionExpired               WebhookEventType = "billing.subscription.expired"
	WebhookEventBillingSubscriptionRenewalFailed         WebhookEventType = "billing.subscription.renewal_failed"
	WebhookEventBillingSubscriptionPlanChanged           WebhookEventType = "billing.subscription.plan_changed"
	WebhookEventBillingSubscriptionPlanChangeScheduled   WebhookEventType = "billing.subscription.plan_change_scheduled"
	WebhookEventBillingSubscriptionPlanChangeCancelled   WebhookEventType = "billing.subscription.plan_change_cancelled"
	WebhookEventBillingSubscriptionTrialEnding           WebhookEventType = "billing.subscription.trial_ending"
	WebhookEventBillingStatementReady                    WebhookEventType = "billing.statement.ready"
	WebhookEventCollectionPaymentReceived                WebhookEventType = "collection.payment.received"
	WebhookEventCollectionPaymentReconciled              WebhookEventType = "collection.payment.reconciled"
	WebhookEventCollectionPaymentApplied                 WebhookEventType = "collection.payment.applied"
	WebhookEventCollectionSettlementUpdated              WebhookEventType = "collection.settlement.updated"
	WebhookEventWebhookEndpointPinged                    WebhookEventType = "webhook_endpoint.pinged"
	WebhookEventWebhookEndpointCreated                   WebhookEventType = "webhook_endpoint.created"
	WebhookEventWebhookEndpointUpdated                   WebhookEventType = "webhook_endpoint.updated"
	WebhookEventWebhookEndpointDeleted                   WebhookEventType = "webhook_endpoint.deleted"
	WebhookEventWebhookEndpointSecretRotated             WebhookEventType = "webhook_endpoint.secret_rotated"
	WebhookEventWebhookEndpointFailing                   WebhookEventType = "webhook_endpoint.failing"
	WebhookEventWebhookEndpointDisabled                  WebhookEventType = "webhook_endpoint.disabled"
	WebhookEventWebhookEndpointEnabled                   WebhookEventType = "webhook_endpoint.enabled"
	WebhookEventWebhookEndpointRecovered                 WebhookEventType = "webhook_endpoint.recovered"
	WebhookEventWebhookEndpointFlagged                   WebhookEventType = "webhook_endpoint.flagged"
)

// WebhookEvent is the body of a webhook delivery. It is serialised once and every attempt sends the
// same bytes. Decode Data with DecodeData, or read it as a map with DataMap.
type WebhookEvent struct {
	// ID is the event id: the same across every retry, redelivery and endpoint. Deduplicate on it.
	ID string `json:"id"`
	// Type is a catalogue type, e.g. `invoice.signed`.
	Type WebhookEventType `json:"type"`
	// Version is the version of Data for this type.
	Version int64 `json:"version"`
	// CreatedAt is when the fact happened (not the delivery time), ISO 8601.
	CreatedAt string `json:"createdAt"`
	// Mode is `sandbox` or `live`.
	Mode Mode `json:"mode"`
	// OrganizationID is the organisation the event is about.
	OrganizationID string `json:"organizationId"`
	// Data is the type's payload, still as JSON.
	Data json.RawMessage `json:"data"`
	// Test is present, and true, only on a test delivery.
	Test bool `json:"test,omitempty"`
}

// DecodeData unmarshals the payload into v, e.g. a struct with the fields of this type.
func (e *WebhookEvent) DecodeData(v any) error {
	return json.Unmarshal(e.Data, v)
}

// DataMap returns the payload as a map, or nil when it is not a JSON object.
func (e *WebhookEvent) DataMap() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(e.Data, &m); err != nil {
		return nil
	}
	return m
}
