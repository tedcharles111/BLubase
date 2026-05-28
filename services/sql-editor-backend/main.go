// Uses pgx to connect to the target project schema directly.
// Reads project ref from JWT claims (anon or user token) and sets search_path.
func runSQLHandler(w http.ResponseWriter, r *http.Request) {
    claims := getJWTClaims(r)
    projectRef := claims["ref"].(string)
    sqlQuery := r.URL.Query().Get("query")

    // Connect to the actual project database/schema (use same cluster or separate)
    conn, err := pool.Acquire(context.Background())
    defer conn.Release()
    _, err = conn.Exec(context.Background(), fmt.Sprintf("SET search_path TO %s", projectRef))
    rows, err := conn.Query(context.Background(), sqlQuery)
    defer rows.Close()

    columns := rows.FieldDescriptions()
    var results []map[string]interface{}
    for rows.Next() {
        values, _ := rows.Values()
        rowMap := map[string]interface{}{}
        for i, col := range columns {
            rowMap[string(col.Name)] = values[i]
        }
        results = append(results, rowMap)
    }
    json.NewEncoder(w).Encode(results)
}
