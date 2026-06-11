// ... (full file is long, so we'll just replace the relevant parts)

// In the oauthCallbackHandler, after fetching the provider's configuration,
// we read the extra settings and apply them.

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}

	var skipNonce, allowNoEmail bool
	var customCallbackURL string
	err := dbPool.QueryRow(context.Background(),
		`SELECT skip_nonce_check, allow_users_without_email, callback_url FROM oauth_providers WHERE provider=$1`,
		provider).Scan(&skipNonce, &allowNoEmail, &customCallbackURL)
	if err != nil {
		skipNonce = false
		allowNoEmail = false
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if !skipNonce {
		val, _ := redisClient.Get(context.Background(), "oauth:"+state).Result()
		if val != provider {
			http.Error(w, "invalid state", 400)
			return
		}
		redisClient.Del(context.Background(), "oauth:"+state)
	}

	// Build redirect URL
	redirectURL := customCallbackURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/auth/%s/callback", os.Getenv("RENDER_EXTERNAL_URL"), provider)
	}

	// Use the correct redirect URL for token exchange
	conf := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint:     config.Endpoint,
	}
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), 500)
		return
	}

	var email string
	switch provider {
	case "github":
		req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		resp, _ := http.DefaultClient.Do(req)
		defer resp.Body.Close()
		var emails []struct{ Email string; Primary bool }
		json.NewDecoder(resp.Body).Decode(&emails)
		for _, e := range emails {
			if e.Primary {
				email = e.Email
				break
			}
		}
		if email == "" && len(emails) > 0 {
			email = emails[0].Email
		}
	case "google":
		resp, _ := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
		defer resp.Body.Close()
		var guser struct{ Email string }
		json.NewDecoder(resp.Body).Decode(&guser)
		email = guser.Email
	}

	if email == "" && !allowNoEmail {
		http.Error(w, "could not fetch email", 500)
		return
	}
	if email == "" && allowNoEmail {
		email = fmt.Sprintf("%s-user@blubase.local", provider)
	}

	var userID string
	err = dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`,
		email).Scan(&userID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}

	claims := jwt.MapClaims{
		"sub": userID, "email": email,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

