package domain

type PhoneApp struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Package  string `json:"package"`
	Category string `json:"category"` // social | system
}

var phoneAppsCatalog = []PhoneApp{
	{ID: "tiktok", Name: "TikTok", Package: "com.zhiliaoapp.musically", Category: "social"},
	{ID: "instagram", Name: "Instagram", Package: "com.instagram.android", Category: "social"},
	{ID: "youtube", Name: "YouTube", Package: "com.google.android.youtube", Category: "social"},
	{ID: "telegram", Name: "Telegram", Package: "org.telegram.messenger", Category: "social"},
	{ID: "facebook", Name: "Facebook", Package: "com.facebook.katana", Category: "social"},
	{ID: "twitter", Name: "X (Twitter)", Package: "com.twitter.android", Category: "social"},

	{ID: "gmail", Name: "Gmail", Package: "com.google.android.gm", Category: "system"},
	{ID: "chrome", Name: "Chrome", Package: "com.android.chrome", Category: "system"},
	{ID: "v2rayng", Name: "V2RayNG", Package: "com.v2ray.ang", Category: "system"},
	{ID: "play_store", Name: "Google Play", Package: "com.android.vending", Category: "system"},
	{ID: "settings", Name: "Настройки", Package: "com.android.settings", Category: "system"},
	{ID: "myfiles", Name: "My Files", Package: "com.sec.android.app.myfiles", Category: "system"},
}

func PhoneAppsByCategory(category string) []PhoneApp {
	var out []PhoneApp
	for _, app := range phoneAppsCatalog {
		if app.Category == category {
			out = append(out, app)
		}
	}
	return out
}
