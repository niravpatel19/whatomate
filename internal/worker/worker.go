package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// Worker processes jobs from the queue
type Worker struct {
	Config    *config.Config
	DB        *gorm.DB
	Redis     *redis.Client
	Log       logf.Logger
	WhatsApp  *whatsapp.Client
	Consumer  *queue.RedisConsumer
	Publisher *queue.Publisher
}

// New creates a new Worker instance
func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client, log logf.Logger) (*Worker, error) {
	consumer, err := queue.NewRedisConsumer(rdb, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	publisher := queue.NewPublisher(rdb, log)

	return &Worker{
		Config:    cfg,
		DB:        db,
		Redis:     rdb,
		Log:       log,
		WhatsApp:  whatsapp.New(log),
		Consumer:  consumer,
		Publisher: publisher,
	}, nil
}

// Run starts the worker and processes jobs until context is cancelled
func (w *Worker) Run(ctx context.Context) error {
	w.Log.Info("Worker starting")

	err := w.Consumer.Consume(ctx, w.handleCampaignJob)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("consumer error: %w", err)
	}

	w.Log.Info("Worker stopped")
	return nil
}

// handleCampaignJob processes a single campaign job
func (w *Worker) handleCampaignJob(ctx context.Context, job *queue.CampaignJob) error {
	w.Log.Info("Processing campaign job", "campaign_id", job.CampaignID)

	if err := w.processCampaign(ctx, job.CampaignID); err != nil {
		w.Log.Error("Failed to process campaign", "error", err, "campaign_id", job.CampaignID)
		return err
	}

	w.Log.Info("Campaign job completed", "campaign_id", job.CampaignID)
	return nil
}

// processCampaign processes a campaign by sending messages to all recipients
func (w *Worker) processCampaign(ctx context.Context, campaignID uuid.UUID) error {
	w.Log.Info("Processing campaign", "campaign_id", campaignID)

	// Get campaign with template
	var campaign models.BulkMessageCampaign
	if err := w.DB.Where("id = ?", campaignID).Preload("Template").First(&campaign).Error; err != nil {
		w.Log.Error("Failed to load campaign for processing", "error", err, "campaign_id", campaignID)
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	// Check if campaign is still in a startable state
	if campaign.Status != "queued" && campaign.Status != "processing" {
		w.Log.Info("Campaign not in processable state", "campaign_id", campaignID, "status", campaign.Status)
		return nil // Not an error, just skip
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := w.DB.Where("name = ? AND organization_id = ?", campaign.WhatsAppAccount, campaign.OrganizationID).First(&account).Error; err != nil {
		w.Log.Error("Failed to load WhatsApp account", "error", err, "account_name", campaign.WhatsAppAccount)
		w.DB.Model(&campaign).Update("status", "failed")
		return fmt.Errorf("failed to load WhatsApp account: %w", err)
	}

	// Update status to processing
	w.DB.Model(&campaign).Update("status", "processing")

	// Get all pending recipients
	var recipients []models.BulkMessageRecipient
	if err := w.DB.Where("campaign_id = ? AND status = ?", campaignID, "pending").Find(&recipients).Error; err != nil {
		w.Log.Error("Failed to load recipients", "error", err, "campaign_id", campaignID)
		w.DB.Model(&campaign).Update("status", "failed")
		return fmt.Errorf("failed to load recipients: %w", err)
	}

	w.Log.Info("Processing recipients", "campaign_id", campaignID, "count", len(recipients))

	sentCount := campaign.SentCount
	failedCount := campaign.FailedCount

	for _, recipient := range recipients {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			w.Log.Info("Campaign processing cancelled by context", "campaign_id", campaignID)
			return ctx.Err()
		default:
		}

		// Check if campaign is still active (not paused/cancelled)
		var currentCampaign models.BulkMessageCampaign
		w.DB.Where("id = ?", campaignID).First(&currentCampaign)
		if currentCampaign.Status == "paused" || currentCampaign.Status == "cancelled" {
			w.Log.Info("Campaign stopped", "campaign_id", campaignID, "status", currentCampaign.Status)
			return nil
		}

		// Get or create contact for this recipient
		contact, err := w.getOrCreateContact(campaign.OrganizationID, recipient.PhoneNumber, recipient.RecipientName)
		if err != nil || contact == nil {
			w.Log.Error("Failed to get or create contact", "error", err, "phone", recipient.PhoneNumber)
			w.DB.Model(&recipient).Updates(map[string]interface{}{
				"status":        "failed",
				"error_message": "Failed to create contact",
			})
			failedCount++
			continue
		}

		// Send template message
		waMessageID, err := w.sendTemplateMessage(ctx, &account, campaign.Template, &recipient)

		// Create Message record with campaign_id in metadata
		message := models.Message{
			OrganizationID:    campaign.OrganizationID,
			WhatsAppAccount:   campaign.WhatsAppAccount,
			ContactID:         contact.ID,
			WhatsAppMessageID: waMessageID,
			Direction:         "outgoing",
			MessageType:       "template",
			TemplateParams:    recipient.TemplateParams,
			Metadata: models.JSONB{
				"campaign_id":    campaignID.String(),
				"recipient_name": recipient.RecipientName,
			},
		}
		if campaign.Template != nil {
			message.TemplateName = campaign.Template.Name
			// Store template body with substituted values for display in chat
			content := campaign.Template.BodyContent
			// Replace placeholders {{1}}, {{2}}, etc. with actual values
			if recipient.TemplateParams != nil {
				for i := 1; i <= 10; i++ {
					key := fmt.Sprintf("%d", i)
					if val, ok := recipient.TemplateParams[key]; ok {
						placeholder := fmt.Sprintf("{{%d}}", i)
						content = strings.ReplaceAll(content, placeholder, fmt.Sprintf("%v", val))
					}
				}
			}
			message.Content = content
		}

		if err != nil {
			w.Log.Error("Failed to send message", "error", err, "recipient", recipient.PhoneNumber)
			message.Status = "failed"
			message.ErrorMessage = err.Error()
			failedCount++
		} else {
			w.Log.Info("Message sent", "recipient", recipient.PhoneNumber, "message_id", waMessageID)
			message.Status = "sent"
			sentCount++
		}

		// Save message record
		if err := w.DB.Create(&message).Error; err != nil {
			w.Log.Error("Failed to save campaign message", "error", err, "recipient", recipient.PhoneNumber)
		}

		// Update BulkMessageRecipient status to track which recipients have been processed
		recipientUpdate := map[string]interface{}{
			"status":               message.Status,
			"whats_app_message_id": waMessageID,
		}
		if message.Status == "failed" {
			recipientUpdate["error_message"] = message.ErrorMessage
		} else {
			recipientUpdate["sent_at"] = time.Now()
		}
		w.DB.Model(&recipient).Updates(recipientUpdate)

		// Update campaign counts
		w.DB.Model(&campaign).Updates(map[string]interface{}{
			"sent_count":   sentCount,
			"failed_count": failedCount,
		})

		// Publish stats update via Redis pub/sub for real-time WebSocket broadcast
		w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
			CampaignID:     campaignID.String(),
			OrganizationID: campaign.OrganizationID,
			Status:         "processing",
			SentCount:      sentCount,
			DeliveredCount: 0,
			ReadCount:      0,
			FailedCount:    failedCount,
		})

		// Small delay to avoid rate limiting (WhatsApp has rate limits)
		time.Sleep(100 * time.Millisecond)
	}

	// Mark campaign as completed
	now := time.Now()
	w.DB.Model(&campaign).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": now,
		"sent_count":   sentCount,
		"failed_count": failedCount,
	})

	// Publish completion status via Redis pub/sub
	w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
		CampaignID:     campaignID.String(),
		OrganizationID: campaign.OrganizationID,
		Status:         "completed",
		SentCount:      sentCount,
		DeliveredCount: 0,
		ReadCount:      0,
		FailedCount:    failedCount,
	})

	w.Log.Info("Campaign completed", "campaign_id", campaignID, "sent", sentCount, "failed", failedCount)
	return nil
}

// sendTemplateMessage sends a template message via WhatsApp Cloud API
func (w *Worker) sendTemplateMessage(ctx context.Context, account *models.WhatsAppAccount, template *models.Template, recipient *models.BulkMessageRecipient) (string, error) {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Build template components with parameters
	var components []map[string]interface{}

	// Add body parameters if template has variables
	if recipient.TemplateParams != nil && len(recipient.TemplateParams) > 0 {
		bodyParams := []map[string]interface{}{}
		for i := 1; i <= 10; i++ {
			key := fmt.Sprintf("%d", i)
			if val, ok := recipient.TemplateParams[key]; ok {
				bodyParams = append(bodyParams, map[string]interface{}{
					"type": "text",
					"text": val,
				})
			}
		}
		if len(bodyParams) > 0 {
			components = append(components, map[string]interface{}{
				"type":       "body",
				"parameters": bodyParams,
			})
		}
	}

	return w.WhatsApp.SendTemplateMessageWithComponents(ctx, waAccount, recipient.PhoneNumber, template.Name, template.Language, components)
}

// Close cleans up worker resources
func (w *Worker) Close() error {
	if w.Consumer != nil {
		return w.Consumer.Close()
	}
	return nil
}

// getOrCreateContact finds or creates a contact for a phone number
func (w *Worker) getOrCreateContact(orgID uuid.UUID, phoneNumber, name string) (*models.Contact, error) {
	// Normalize phone number (remove + prefix if present)
	normalizedPhone := phoneNumber
	if len(normalizedPhone) > 0 && normalizedPhone[0] == '+' {
		normalizedPhone = normalizedPhone[1:]
	}

	// Try to find existing contact
	var contact models.Contact
	err := w.DB.Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&contact).Error
	if err == nil {
		return &contact, nil
	}

	// Also try with + prefix
	err = w.DB.Where("organization_id = ? AND phone_number = ?", orgID, "+"+normalizedPhone).First(&contact).Error
	if err == nil {
		return &contact, nil
	}

	// Create new contact
	contact = models.Contact{
		OrganizationID: orgID,
		PhoneNumber:    normalizedPhone,
		ProfileName:    name,
	}
	if err := w.DB.Create(&contact).Error; err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	w.Log.Info("Created new contact for campaign recipient", "phone", normalizedPhone, "name", name)
	return &contact, nil
}
