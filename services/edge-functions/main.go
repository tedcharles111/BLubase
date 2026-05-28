func invokeFunction(w http.ResponseWriter, r *http.Request) {
    functionName := chi.URLParam(r, "name")
    // Fetch function code from MinIO (key: functions/<project_ref>/<name>/index.ts)
    code, _ := minioClient.GetObject(...)
    // Write code to temp file
    // Execute: deno run --allow-net --allow-env tempFile.ts
    cmd := exec.Command("deno", "run", "--allow-net", "--allow-env", tempFile)
    cmd.Stdin = r.Body
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &out
    err := cmd.Run()
    w.Write(out.Bytes())
}
