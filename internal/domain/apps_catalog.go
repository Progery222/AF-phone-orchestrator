package domain

type PhoneApp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Package string `json:"package"`
}

var phoneAppsCatalog = []PhoneApp{
	{ID: "gmail", Name: "Gmail", Package: "com.google.android.gm"},
	{ID: "chrome", Name: "Chrome", Package: "com.android.chrome"},
	{ID: "tiktok", Name: "TikTok", Package: "com.zhiliaoapp.musically"},
	{ID: "tiktok_trill", Name: "TikTok (Trill)", Package: "com.ss.android.ugc.trill"},
	{ID: "v2rayng", Name: "V2RayNG", Package: "com.v2ray.ang"},
	{ID: "play_store", Name: "Google Play", Package: "com.android.vending"},
	{ID: "settings", Name: "Настройки", Package: "com.android.settings"},
	{ID: "myfiles", Name: "My Files", Package: "com.sec.android.app.myfiles"},
}

func PhoneAppsList() []PhoneApp {
	out := make([]PhoneApp, len(phoneAppsCatalog))
	copy(out, phoneAppsCatalog)
	return out
}
