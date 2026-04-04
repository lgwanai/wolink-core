package services

import (
	"regexp"

	"wolink-core/internal/config"
)

type SecurityService struct {
	sensitivePatterns   []*regexp.Regexp
	replacementPatterns []string
}

func NewSecurityService(cfg *config.Config) *SecurityService {
	service := &SecurityService{
		replacementPatterns: cfg.Security.ReplacementPatterns,
	}
	
	// 编译正则表达式
	for _, pattern := range cfg.Security.SensitivePatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			service.sensitivePatterns = append(service.sensitivePatterns, compiled)
		}
	}
	
	return service
}

// DetectAndReplaceSensitiveInfo 检测并替换敏感信息
func (s *SecurityService) DetectAndReplaceSensitiveInfo(text string) (cleanText string, hasSensitive bool, sensitiveTypes []string) {
	cleanText = text
	
	for i, pattern := range s.sensitivePatterns {
		if pattern.MatchString(cleanText) {
			hasSensitive = true
			
			// 记录敏感信息类型
			var sensitiveType string
			switch i {
			case 0:
				sensitiveType = "phone"
			case 1:
				sensitiveType = "id_card"
			case 2:
				sensitiveType = "bank_card"
			default:
				sensitiveType = "unknown"
			}
			sensitiveTypes = append(sensitiveTypes, sensitiveType)
			
			// 替换敏感信息
			if i < len(s.replacementPatterns) {
				cleanText = pattern.ReplaceAllString(cleanText, s.replacementPatterns[i])
			}
		}
	}
	
	return cleanText, hasSensitive, sensitiveTypes
}