package admincomponents

import (
	"fmt"
	"placeholder_project_tag/internal/data"
	"strings"
	"time"
)

// TODO: get tz from request? And convert to that
func ConvertToLocalTZ(t time.Time) time.Time {
	tz, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return t
	}
	return t.In(tz)
}

// timeAgo returns a human-readable relative time string
func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	diff := time.Since(ConvertToLocalTZ(t))
	switch {
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 48*time.Hour:
		return "Yesterday"
	case diff < 7*24*time.Hour:
		d := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", d)
	default:
		return ConvertToLocalTZ(t).Format("Jan 2, 2006")
	}
}

// permissionSummary returns a short summary like "2 user · 1 admin"
func PermissionSummary(permissions []data.Permission) string {
	userCount := 0
	adminCount := 0
	for _, p := range permissions {
		s := string(p)
		if strings.HasPrefix(s, "admin:") {
			adminCount++
		} else {
			userCount++
		}
	}
	parts := []string{}
	if userCount > 0 {
		parts = append(parts, fmt.Sprintf("%d user", userCount))
	}
	if adminCount > 0 {
		parts = append(parts, fmt.Sprintf("%d admin", adminCount))
	}
	if len(parts) == 0 {
		return "No permissions"
	}
	return strings.Join(parts, " · ")
}

// permissionBadgeClass returns a CSS class based on permission type
func PermissionBadgeClass(p string) string {
	if strings.HasPrefix(p, "admin:") {
		return "badge badge--admin"
	}
	return "badge badge--user"
}
