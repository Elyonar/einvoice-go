package einvoice

import "context"

// WebhookEndpointsService is webhook endpoints (`/n/v1/webhook-endpoints`). An API key may read and
// test them; creating, changing, deleting and rotating an endpoint's secret are done in the dashboard.
type WebhookEndpointsService struct{ baseService }

// List is the endpoints of the key's organisation and mode; never a secret (`webhook.endpoint.read`).
func (s *WebhookEndpointsService) List(ctx context.Context, opts ...RequestOption) ([]ListWebhookEndpointsItem, error) {
	out := []ListWebhookEndpointsItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/n/v1/webhook-endpoints", Options: requestOptions(opts)}, &out)
	return out, err
}

// Get is one endpoint, never its secret (`webhook.endpoint.read`).
func (s *WebhookEndpointsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetWebhookEndpointData, error) {
	out := &GetWebhookEndpointData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/n/v1/webhook-endpoints/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Test sends one test delivery (`"test": true`; never charged; `webhook.endpoint.test`). A nil params sends `{}`.
func (s *WebhookEndpointsService) Test(ctx context.Context, id string, params *TestWebhookEndpointBody, opts ...RequestOption) (*TestWebhookEndpointData, error) {
	var body any = params
	if params == nil {
		body = struct{}{}
	}
	out := &TestWebhookEndpointData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/n/v1/webhook-endpoints/" + seg(id) + "/test", Body: body, Options: requestOptions(opts)}, out)
	return out, err
}

// WebhookDeliveriesService is webhook deliveries (`/n/v1/webhook-deliveries`): one per event ×
// endpoint, with its attempts.
type WebhookDeliveriesService struct{ baseService }

// List lists deliveries by endpoint, status or type (`webhook.delivery.read`).
func (s *WebhookDeliveriesService) List(ctx context.Context, query *ListWebhookDeliveriesQuery, opts ...RequestOption) (*Page[ListWebhookDeliveriesItem], error) {
	return page[ListWebhookDeliveriesItem](ctx, &s.baseService, Call{Method: "GET", Path: "/n/v1/webhook-deliveries", Query: query, Options: requestOptions(opts)})
}

// Get is one delivery and its attempt log (`webhook.delivery.read`).
func (s *WebhookDeliveriesService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetWebhookDeliveryData, error) {
	out := &GetWebhookDeliveryData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/n/v1/webhook-deliveries/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Redeliver is one more attempt with the stored bytes (`webhook.delivery.redeliver`; 202).
func (s *WebhookDeliveriesService) Redeliver(ctx context.Context, id string, opts ...RequestOption) (*RedeliverWebhookDeliveryData, error) {
	out := &RedeliverWebhookDeliveryData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/n/v1/webhook-deliveries/" + seg(id) + "/redeliver", Options: requestOptions(opts)}, out)
	return out, err
}

// WebhookEventsService is logged webhook events (`/n/v1/webhook-events`, last 30 days).
type WebhookEventsService struct{ baseService }

// List lists events of the key's mode plus organisation-scoped ones (`webhook.delivery.read`).
func (s *WebhookEventsService) List(ctx context.Context, query *ListWebhookEventsQuery, opts ...RequestOption) (*Page[ListWebhookEventsItem], error) {
	return page[ListWebhookEventsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/n/v1/webhook-events", Query: query, Options: requestOptions(opts)})
}

// Get is one event with its public `Data` (`webhook.delivery.read`).
func (s *WebhookEventsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetWebhookEventData, error) {
	out := &GetWebhookEventData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/n/v1/webhook-events/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Redeliver replays an event to one endpoint (`webhook.delivery.redeliver`; 202).
func (s *WebhookEventsService) Redeliver(ctx context.Context, id string, params *RedeliverWebhookEventBody, opts ...RequestOption) (*RedeliverWebhookEventData, error) {
	out := &RedeliverWebhookEventData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/n/v1/webhook-events/" + seg(id) + "/redeliver", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// WebhookEventTypesService is the webhook event catalogue (`/n/v1/webhook-event-types`).
type WebhookEventTypesService struct{ baseService }

// List is every event type, with whether this key may subscribe to it.
func (s *WebhookEventTypesService) List(ctx context.Context, opts ...RequestOption) ([]ListWebhookEventTypesItem, error) {
	out := []ListWebhookEventTypesItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/n/v1/webhook-event-types", Options: requestOptions(opts)}, &out)
	return out, err
}

// WebhooksService is webhooks (`/n/v1`), grouped as the API tags it. Verify deliveries with VerifyWebhook.
type WebhooksService struct {
	// Endpoints (read and test).
	Endpoints *WebhookEndpointsService
	// Deliveries and redelivery.
	Deliveries *WebhookDeliveriesService
	// Events: logged events and replay.
	Events *WebhookEventsService
	// EventTypes: the event catalogue.
	EventTypes *WebhookEventTypesService
}

func newWebhooksService(base baseService) *WebhooksService {
	return &WebhooksService{
		Endpoints:  &WebhookEndpointsService{base},
		Deliveries: &WebhookDeliveriesService{base},
		Events:     &WebhookEventsService{base},
		EventTypes: &WebhookEventTypesService{base},
	}
}
