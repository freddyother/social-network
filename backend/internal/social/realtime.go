package social

import "time"

type EventPublisher interface {
	PublishToUser(userID, eventType string, payload any)
	PublishToPost(postID, eventType string, payload any)
}

type noopEventPublisher struct{}

func (noopEventPublisher) PublishToUser(string, string, any) {}

func (noopEventPublisher) PublishToPost(string, string, any) {}

type NotificationEvent struct {
	Notification Notification `json:"notification"`
}

type NotificationReadEvent struct {
	NotificationID string `json:"notificationId"`
}

type FollowRequestEvent struct {
	RequestID   string    `json:"requestId"`
	SenderID    string    `json:"senderId"`
	RecipientID string    `json:"recipientId"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

type CommentEvent struct {
	PostID  string  `json:"postId"`
	Comment Comment `json:"comment"`
}

type notificationDelivery struct {
	UserID       string
	Notification Notification
}
