package cookie

import "net/http"

func NewCookie(name string, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: name, Value: value, Path: "/",
		MaxAge: maxAge, HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func NewDeleteCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name: name, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode,
	}
}
