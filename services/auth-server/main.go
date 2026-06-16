// ... (the full file is huge, so we'll patch only the forgotPasswordHandler)

// Add this function before forgotPasswordHandler
func sendEmail(to, subject, body string) error {
	// Fetch SMTP settings from database
	rows, err := dbPool.Query(context.Background(), `SELECT key, value FROM smtp_config`)
	if err != nil {
		return err
	}
	defer rows.Close()
	cfg := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		cfg[k] = v
	}
	// Expecting keys: host, port, username, password, sender_email, sender_name
	host := cfg["host"]
	port := cfg["port"]
	username := cfg["username"]
	password := cfg["password"]
	senderEmail := cfg["sender_email"]
	if senderEmail == "" { senderEmail = "noreply@blubase.dev" }

	if host == "" || port == "" || username == "" || password == "" {
		return fmt.Errorf("SMTP not configured")
	}

	auth := smtp.PlainAuth("", username, password, host)
	msg := []byte("To: " + to + "\r\n" +
		"From: " + senderEmail + "\r\n" +
		"Subject: " + subject + "\r\n\r\n" +
		body)
	err = smtp.SendMail(host+":"+port, auth, senderEmail, []string{to}, msg)
	return err
}

// Then update the forgotPasswordHandler:
func sendEmail(to, subject, body string) error {
	rows, err := dbPool.Query(context.Background(), `SELECT key, value FROM smtp_config`)
	if err != nil { return err }
	defer rows.Close()
	cfg := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		cfg[k] = v
	}
	host := cfg["host"]
	port := cfg["port"]
	username := cfg["username"]
	password := cfg["password"]
	senderEmail := cfg["sender_email"]
	if senderEmail == "" { senderEmail = "noreply@blubase.dev" }
	if host == "" || port == "" || username == "" || password == "" {
		return fmt.Errorf("SMTP not configured")
	}
	auth := smtp.PlainAuth("", username, password, host)
	msg := []byte("To: " + to + "
" +
		"From: " + senderEmail + "
" +
		"Subject: " + subject + "

" +
		body)
	return smtp.SendMail(host+":"+port, auth, senderEmail, []string{to}, msg)
}

func forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string `json:"email"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" {
		http.Error(w, `{"error":"email required"}`, 400)
		return
	}
	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	redisClient.Set(context.Background(), "reset:"+req.Email, otp, 15*time.Minute)

	// Send email
	err := sendEmail(req.Email, "Password Reset Code", fmt.Sprintf("Your Blubase password reset code is: %s", otp))
	if err != nil {
		log.Printf("Failed to send email: %v", err)
		// Still return success to prevent email enumeration
	}
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If that email exists, a reset code has been sent",
	})

