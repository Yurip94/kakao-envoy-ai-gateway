package extproc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/memory"
	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/openai"
)

type sessionIDContextKey struct{}

type Processor struct {
	extprocv3.UnimplementedExternalProcessorServer
	Config Config
	Store  memory.Store
}

func NewProcessor(cfg Config, store memory.Store) *Processor {
	return &Processor{
		Config: cfg,
		Store:  store,
	}
}

func (p *Processor) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		resp, nextCtx, err := p.handleRequest(ctx, req)
		if err != nil {
			return err
		}
		ctx = nextCtx

		if resp == nil {
			continue
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (p *Processor) handleRequest(ctx context.Context, req *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, context.Context, error) {
	switch v := req.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		sessionID := headerValue(v.RequestHeaders, p.Config.SessionHeader)
		if sessionID == "" {
			if !p.isMissingSessionPassThrough() {
				return nil, ctx, memory.ErrMissingSessionID
			}
			return continueRequestHeaders(), ctx, nil
		}
		return continueRequestHeaders(), context.WithValue(ctx, sessionIDContextKey{}, sessionID), nil

	case *extprocv3.ProcessingRequest_RequestBody:
		return p.handleRequestBody(ctx, v.RequestBody)

	case *extprocv3.ProcessingRequest_ResponseBody:
		return p.handleResponseBody(ctx, v.ResponseBody)

	case *extprocv3.ProcessingRequest_ResponseHeaders:
		return continueResponseHeaders(), ctx, nil

	case *extprocv3.ProcessingRequest_RequestTrailers:
		return continueRequestTrailers(), ctx, nil

	case *extprocv3.ProcessingRequest_ResponseTrailers:
		return continueResponseTrailers(), ctx, nil

	default:
		return nil, ctx, fmt.Errorf("unsupported processing request type %T", v)
	}
}

func (p *Processor) handleRequestBody(ctx context.Context, body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, context.Context, error) {
	sessionID := sessionIDFromContext(ctx)
	if sessionID == "" {
		if !p.isMissingSessionPassThrough() {
			return nil, ctx, memory.ErrMissingSessionID
		}
		return continueRequestBody(nil), ctx, nil
	}

	history, err := p.Store.Load(ctx, sessionID, p.Config.MaxHistoryLength)
	if err != nil {
		if err := p.handleRedisError("load_history", sessionID, err); err != nil {
			return nil, ctx, err
		}
		history = nil
	}

	rawBody := body.GetBody()
	chatReq, err := openai.ParseChatRequest(rawBody)
	if err != nil {
		log.Printf("warn: request body parse failed (pass-through): %v", err)
		return continueRequestBody(nil), ctx, nil
	}

	merged := openai.MergeMessages(history, chatReq.Messages, p.Config.MaxHistoryLength)
	mutatedBody, err := replaceMessagesField(rawBody, merged)
	if err != nil {
		log.Printf("warn: request body mutation failed (pass-through): %v", err)
		return continueRequestBody(nil), ctx, nil
	}

	userMessages := filterByRole(chatReq.Messages, "user")
	if len(userMessages) > 0 {
		if err := p.Store.Append(ctx, sessionID, userMessages, p.Config.MemoryTTL, p.Config.MaxHistoryLength); err != nil {
			if err := p.handleRedisError("append_user_messages", sessionID, err); err != nil {
				return nil, ctx, err
			}
		}
	}

	return continueRequestBody(mutatedBody), ctx, nil
}

func (p *Processor) handleResponseBody(ctx context.Context, body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, context.Context, error) {
	sessionID := sessionIDFromContext(ctx)
	if sessionID == "" {
		if !p.isMissingSessionPassThrough() {
			return nil, ctx, memory.ErrMissingSessionID
		}
		return continueResponseBody(), ctx, nil
	}

	assistant, err := openai.ExtractAssistantMessage(body.GetBody())
	if err != nil {
		log.Printf("warn: assistant message extraction failed (pass-through): %v", err)
		return continueResponseBody(), ctx, nil
	}

	if err := p.Store.Append(ctx, sessionID, []openai.Message{assistant}, p.Config.MemoryTTL, p.Config.MaxHistoryLength); err != nil {
		if err := p.handleRedisError("append_assistant_message", sessionID, err); err != nil {
			return nil, ctx, err
		}
	}

	return continueResponseBody(), ctx, nil
}

func (p *Processor) isMissingSessionPassThrough() bool {
	return strings.EqualFold(p.Config.MissingSessionPolicy, "pass-through")
}

func (p *Processor) isRedisFailOpen() bool {
	return strings.EqualFold(p.Config.RedisFailurePolicy, "fail-open")
}

func (p *Processor) handleRedisError(stage, sessionID string, err error) error {
	if err == nil {
		return nil
	}
	if p.isRedisFailOpen() {
		log.Printf("warn: redis error at %s (session=%s): %v", stage, sessionID, err)
		return nil
	}
	return fmt.Errorf("redis error at %s (session=%s): %w", stage, sessionID, err)
}

func sessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDContextKey{}).(string)
	return strings.TrimSpace(v)
}

func headerValue(headers *extprocv3.HttpHeaders, name string) string {
	if headers == nil || headers.GetHeaders() == nil {
		return ""
	}

	for _, h := range headers.GetHeaders().GetHeaders() {
		if !strings.EqualFold(h.GetKey(), name) {
			continue
		}

		if raw := strings.TrimSpace(string(h.GetRawValue())); raw != "" {
			return raw
		}
		return strings.TrimSpace(h.GetValue())
	}

	return ""
}

func replaceMessagesField(body []byte, messages []openai.Message) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	encodedMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	payload["messages"] = encodedMessages

	return json.Marshal(payload)
}

func filterByRole(messages []openai.Message, role string) []openai.Message {
	out := make([]openai.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == role {
			out = append(out, m)
		}
	}
	return out
}

func continueRequestHeaders() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					Status: extprocv3.CommonResponse_CONTINUE,
				},
			},
		},
	}
}

func continueResponseHeaders() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					Status: extprocv3.CommonResponse_CONTINUE,
				},
			},
		},
	}
}

func continueRequestBody(mutatedBody []byte) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{
		Status: extprocv3.CommonResponse_CONTINUE,
	}
	if len(mutatedBody) > 0 {
		common.Status = extprocv3.CommonResponse_CONTINUE_AND_REPLACE
		common.BodyMutation = &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{
				Body: mutatedBody,
			},
		}
		common.HeaderMutation = &extprocv3.HeaderMutation{
			SetHeaders: []*corev3.HeaderValueOption{
				{
					Header: &corev3.HeaderValue{
						Key:   "content-length",
						Value: fmt.Sprintf("%d", len(mutatedBody)),
					},
				},
			},
		}
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: common,
			},
		},
	}
}

func continueResponseBody() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					Status: extprocv3.CommonResponse_CONTINUE,
				},
			},
		},
	}
}

func continueRequestTrailers() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestTrailers{
			RequestTrailers: &extprocv3.TrailersResponse{
				HeaderMutation: &extprocv3.HeaderMutation{},
			},
		},
	}
}

func continueResponseTrailers() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseTrailers{
			ResponseTrailers: &extprocv3.TrailersResponse{
				HeaderMutation: &extprocv3.HeaderMutation{},
			},
		},
	}
}
