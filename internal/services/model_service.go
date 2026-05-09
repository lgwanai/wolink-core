package services

// ModelService is a placeholder. Model configuration is handled by ModelConfigService.
// In single-node mode, models are read from config files.
// In multi-node mode, models are synced from admin service via AdminSyncService.
type ModelService struct{}

func NewModelService() *ModelService {
	return &ModelService{}
}
