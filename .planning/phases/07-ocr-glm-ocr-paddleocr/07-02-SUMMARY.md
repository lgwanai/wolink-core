---
phase: 07-ocr-glm-ocr-paddleocr
plan: 02
subsystem: plugin-system
tags: [ocr, plugin, interface, models]
dependency_graph:
  requires: []
  provides: [OCRPlugin interface, OCR request/response models, CallOCR implementation]
  affects: [plugin system, model handlers]
tech_stack:
  added:
    - OCRPlugin interface
    - OCRRequest/OCRResponse/OCRRegion models
    - CallOCR multipart file upload
  patterns:
    - Interface segregation pattern (OCRPlugin extends plugin capabilities)
    - Multipart file upload pattern (similar to AudioTranscription)
    - Plugin capability extension
key_files:
  created: []
  modified:
    - internal/plugins/interface.go (added OCRPlugin interface)
    - internal/models/models.go (added OCR models)
    - internal/plugins/plugin_openai.go (added CallOCR method)
decisions:
  - OCRPlugin follows AudioPlugin pattern for consistency
  - OCR models placed in models.go alongside other request/response types
  - CallOCR uses multipart file upload similar to CallAudioTranscription
  - Endpoint configured as /v1/ocr for OCR processing
metrics:
  duration: 3 minutes
  completed_date: 2026-05-05T01:55:57Z
  tasks_completed: 3
  files_modified: 3
  lines_added: 105
  commits: 3
---

# Phase 07 Plan 02: Add OCR Capability to Plugin System Summary

## One-Liner

Added OCR capability to plugin infrastructure with OCRPlugin interface, OCR request/response models, and OpenAI plugin implementation following the AudioPlugin pattern.

## What Changed

### OCRPlugin Interface (Task 1)

Added `OCRPlugin` interface to `internal/plugins/interface.go` after AudioPlugin interface:

```go
type OCRPlugin interface {
	CallOCR(ctx context.Context, config *models.ModelConfig, request *models.OCRRequest) (*models.OCRResponse, error)
}
```

**Design rationale:** Follows the same pattern as AudioPlugin, EmbeddingPlugin, and RerankPlugin for consistency. OCR is similar to audio - processes images (input_modal: image) and returns text.

### OCR Request/Response Models (Task 2)

Added three new model structs to `internal/models/models.go`:

1. **OCRRequest**: File upload with optional language and response format
   - File: interface{} for multipart form handling
   - Model: string for model selection
   - Language: optional language hint
   - ResponseFormat: json or text output

2. **OCRResponse**: Text extraction results
   - Text: extracted text content
   - Language: detected/specified language
   - Regions: structured output with bounding boxes

3. **OCRRegion**: Detailed text detection
   - Text: detected text
   - Confidence: detection confidence score
   - BoundingBox: [x, y, width, height] for text location

**Design rationale:** Follows AudioTranscriptionRequest pattern. OCR models defined alongside audio models since they're both multimodal capabilities.

### CallOCR Implementation (Task 3)

Implemented `CallOCR` method in `internal/plugins/plugin_openai.go`:

**Key implementation details:**
- Multipart file upload for image processing
- Model name resolution (config.Model → request.Model fallback)
- Language and response_format parameter support
- POST to `{baseURL}/v1/ocr` endpoint
- Comprehensive error handling with context
- JSON response parsing into OCRResponse

**Code pattern:** Mirrors CallAudioTranscription implementation with:
- File header handling and streaming
- Form field population
- Proper resource cleanup (defer file.Close())
- Response validation and error wrapping

## Technical Details

### File Upload Pattern

```go
// Add file from multipart form
if fileHeader, ok := request.File.(*multipart.FileHeader); ok {
    file, err := fileHeader.Open()
    // ... error handling
    defer file.Close()
    
    part, err := writer.CreateFormFile("file", fileHeader.Filename)
    // ... streaming copy
}
```

### Model Resolution

```go
model := connConfig.Model
if model == "" {
    model = request.Model
}
```

Allows configuration to override request model, providing flexibility for routing.

### Error Handling

All errors wrapped with context:
- `failed to open file: %w`
- `failed to create form file: %w`
- `failed to send request: %w`
- `api error (status %d): %s`

## Verification

All tasks verified and committed:

```bash
# OCRPlugin interface exists
grep "OCRPlugin interface" internal/plugins/interface.go
# Output: type OCRPlugin interface {

# OCR models defined
grep "OCRRequest" internal/models/models.go
grep "OCRResponse" internal/models/models.go
# Output: type OCRRequest struct {
# Output: type OCRResponse struct {

# Plugin implementation exists
grep "CallOCR" internal/plugins/plugin_openai.go
# Output: func (p *OpenAIPlugin) CallOCR(...)

# Code compiles
go build ./internal/...
# (no errors)
```

## Commits

1. **e390752** - feat(07-02): add OCRPlugin interface
   - Added OCRPlugin interface with CallOCR method
   - Followed AudioPlugin pattern for consistency

2. **2eece58** - feat(07-02): add OCR request/response models
   - Added OCRRequest, OCRResponse, OCRRegion structs
   - Followed AudioTranscriptionRequest pattern

3. **49b913d** - feat(07-02): implement CallOCR in OpenAI plugin
   - Implemented CallOCR method with multipart file upload
   - Added language and response_format support

## Deviations from Plan

None - plan executed exactly as written. All three tasks completed successfully with proper verification and atomic commits.

## Next Steps

Plan 07-03 will:
- Add OCR model handler in internal/api/handlers
- Create OCR API routes
- Integrate OCRPlugin with service layer

## Success Criteria Met

- [x] OCRPlugin interface defined in interface.go
- [x] OCRRequest and OCRResponse models defined
- [x] OpenAI plugin implements CallOCR method
- [x] All code compiles without errors
- [x] Each task committed individually
- [x] SUMMARY.md created

## Self-Check: PASSED

All claims verified:
- ✓ OCRPlugin interface exists in internal/plugins/interface.go
- ✓ OCR models exist in internal/models/models.go
- ✓ CallOCR method exists in internal/plugins/plugin_openai.go
- ✓ All commits exist in git history (e390752, 2eece58, 49b913d)
- ✓ Code compiles successfully

## Self-Check: PASSED

All claims verified:
- ✓ OCRPlugin interface exists in internal/plugins/interface.go
- ✓ OCR models exist in internal/models/models.go
- ✓ CallOCR method exists in internal/plugins/plugin_openai.go
- ✓ All commits exist in git history (e390752, 2eece58, 49b913d, 5fd01bb)
- ✓ Code compiles successfully
- ✓ SUMMARY.md created in plan directory
