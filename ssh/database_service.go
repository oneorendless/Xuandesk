package ssh

import (
	"database/sql"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// DBConnection 鏁版嵁搴撹繛鎺ワ紙閫氳繃 SSH 闅ч亾锛?type DBConnection struct {
	db       *sql.DB
	listener net.Listener
	sshConn  *SSHClient
	dbType   string
	dbHost   string
	dbPort   int
}

// DatabaseService 鏁版嵁搴撴湇鍔★紙Go 鍘熺敓椹卞姩 + SSH 闅ч亾锛?type DatabaseService struct {
	mu      sync.RWMutex
	sshSvc  *SSHService
	conns   map[string]*DBConnection // key = connID:dbType:host:port
}

// NewDatabaseService 鍒涘缓鏁版嵁搴撴湇鍔?func NewDatabaseService(sshSvc *SSHService) *DatabaseService {
	return &DatabaseService{
		sshSvc: sshSvc,
		conns:  make(map[string]*DBConnection),
	}
}

// connKey 鐢熸垚杩炴帴缂撳瓨 key锛堝寘鍚暟鎹簱鍚嶏紝涓嶅悓鏁版嵁搴撶敤涓嶅悓杩炴帴锛?func connKey(connID, dbType, host string, port int, database string) string {
	return fmt.Sprintf("%s:%s:%s:%d:%s", connID, dbType, host, port, database)
}

// openDB 閫氳繃 SSH 闅ч亾鎵撳紑鏁版嵁搴撹繛鎺?func (s *DatabaseService) openDB(connID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) (*sql.DB, error) {
	key := connKey(connID, dbType, dbHost, dbPort, dbDatabase)

	// 浣跨敤鍐欓攣闃叉骞跺彂鍒涘缓鍚屼竴杩炴帴
	s.mu.Lock()

	// 妫€鏌ョ紦瀛?	if existing, ok := s.conns[key]; ok && existing.db != nil {
		db := existing.db
		s.mu.Unlock()
		// 缂撳瓨鍛戒腑锛岀洿鎺ヨ繑鍥烇紙Ping 鍦ㄩ攣澶栨墽琛岄伩鍏嶆閿侊級
		if err := db.Ping(); err == nil {
			return db, nil
		}
		// 杩炴帴宸插け鏁堬紝閲嶆柊鍒涘缓
		s.mu.Lock()
		if old, ok := s.conns[key]; ok {
			old.db.Close()
			if old.listener != nil {
				old.listener.Close()
			}
			delete(s.conns, key)
		}
	}

	// 鑾峰彇 SSH 瀹㈡埛绔?	client, err := s.sshSvc.GetClient(connID)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("SSH 杩炴帴涓嶅瓨鍦? %v", err)
	}
	if !client.IsConnected() {
		s.mu.Unlock()
		return nil, fmt.Errorf("SSH 杩炴帴宸叉柇寮€")
	}

	// 鍒涘缓 SSH 闅ч亾
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("鍒涘缓鏈湴鐩戝惉澶辫触: %v", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port

	// 鍚姩闅ч亾杞彂
	go s.forwardTunnel(listener, client, dbHost, dbPort)

	// 绛夊緟闅ч亾灏辩华
	time.Sleep(100 * time.Millisecond)

	// 鏋勫缓 DSN
	var dsn string
	switch dbType {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?timeout=10s&parseTime=true",
			dbUser, dbPassword, localPort, dbDatabase)
	case "postgresql":
		dsn = fmt.Sprintf("host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=10",
			localPort, dbUser, dbPassword, dbDatabase)
	default:
		listener.Close()
		s.mu.Unlock()
		return nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	// 鎵撳紑鏁版嵁搴撹繛鎺?	db, err := sql.Open(dbType, dsn)
	if err != nil {
		listener.Close()
		s.mu.Unlock()
		return nil, fmt.Errorf("鎵撳紑鏁版嵁搴撳け璐? %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		listener.Close()
		s.mu.Unlock()
		return nil, fmt.Errorf("杩炴帴鏁版嵁搴撳け璐? %v", err)
	}

	// 缂撳瓨杩炴帴
	s.conns[key] = &DBConnection{
		db:       db,
		listener: listener,
		sshConn:  client,
		dbType:   dbType,
		dbHost:   dbHost,
		dbPort:   dbPort,
	}
	s.mu.Unlock()

	return db, nil
}

// forwardTunnel SSH 闅ч亾杞彂
func (s *DatabaseService) forwardTunnel(listener net.Listener, client *SSHClient, remoteHost string, remotePort int) {
	remoteAddr := fmt.Sprintf("%s:%d", remoteHost, remotePort)

	for {
		localConn, err := listener.Accept()
		if err != nil {
			return // listener 宸插叧闂?		}

		remoteConn, err := client.client.Dial("tcp", remoteAddr)
		if err != nil {
			localConn.Close()
			continue
		}

		go func() {
			defer localConn.Close()
			defer remoteConn.Close()
			done := make(chan struct{})
			go func() {
				buf := make([]byte, 32*1024)
				io.CopyBuffer(remoteConn, localConn, buf)
				close(done)
			}()
			buf := make([]byte, 32*1024)
			io.CopyBuffer(localConn, remoteConn, buf)
			<-done
		}()
	}
}

// CloseDB 鍏抽棴鎸囧畾鏁版嵁搴撹繛鎺?func (s *DatabaseService) CloseDB(connID, dbType, dbHost string, dbPort int, dbDatabase string) {
	key := connKey(connID, dbType, dbHost, dbPort, dbDatabase)
	s.mu.Lock()
	if existing, ok := s.conns[key]; ok {
		if existing.db != nil {
			existing.db.Close()
		}
		if existing.listener != nil {
			existing.listener.Close()
		}
		delete(s.conns, key)
	}
	s.mu.Unlock()
}

// ========== 鍏叡 API ==========

// TestConnection 娴嬭瘯鏁版嵁搴撹繛鎺?func (s *DatabaseService) TestConnection(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return err
	}
	return db.Ping()
}

// ListDatabases 鍒楀嚭鏁版嵁搴?func (s *DatabaseService) ListDatabases(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword string) ([]string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = "SHOW DATABASES"
	case "postgresql":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
	default:
		return nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		databases = append(databases, name)
	}
	return databases, nil
}

// ListTables 鍒楀嚭琛?func (s *DatabaseService) ListTables(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) ([]string, error) {
	log.Info("[DB] ListTables", "sshConnID", sshConnID, "dbType", dbType, "dbHost", dbHost, "dbPort", dbPort, "dbUser", dbUser, "dbDatabase", dbDatabase)

	if dbDatabase == "" {
		return nil, fmt.Errorf("鏈€夋嫨鏁版嵁搴?)
	}

	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = "SHOW TABLES"
	case "postgresql":
		query = "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
	default:
		return nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	return tables, nil
}

// GetTableStructure 鑾峰彇琛ㄧ粨鏋?func (s *DatabaseService) GetTableStructure(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string) ([][]string, []string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return nil, nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = fmt.Sprintf("DESCRIBE `%s`", tableName)
	case "postgresql":
		query = fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable, column_default, character_maximum_length
			FROM information_schema.columns
			WHERE table_name = '%s' AND table_schema = 'public'
			ORDER BY ordinal_position`, tableName)
	default:
		return nil, nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	return s.queryToRows(db, query)
}

// GetTableData 娴忚琛ㄦ暟鎹紙鍒嗛〉锛?func (s *DatabaseService) GetTableData(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string, page, pageSize int) ([][]string, []string, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		log.Error("[DB] GetTableData openDB 澶辫触", "error", err)
		return nil, nil, err
	}

	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d", tableName, pageSize, offset)
	log.Info("[DB] GetTableData", "query", query)

	rows, columns, err := s.queryToRows(db, query)
	if err != nil {
		log.Error("[DB] GetTableData 鏌ヨ澶辫触", "error", err, "query", query)
		return nil, nil, err
	}

	log.Info("[DB] GetTableData 缁撴灉", "rows", len(rows), "columns", len(columns), "query", query)
	return rows, columns, nil
}

// GetTableCount 鑾峰彇琛ㄨ褰曟暟
func (s *DatabaseService) GetTableCount(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string) (int64, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return 0, err
	}

	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	err = db.QueryRow(query).Scan(&count)
	return count, err
}

// GetTableIndexes 鑾峰彇琛ㄧ储寮?func (s *DatabaseService) GetTableIndexes(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string) ([][]string, []string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return nil, nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = fmt.Sprintf("SHOW INDEX FROM `%s`", tableName)
	case "postgresql":
		query = fmt.Sprintf(`
			SELECT indexname, indexdef
			FROM pg_indexes
			WHERE tablename = '%s' AND schemaname = 'public'`, tableName)
	default:
		return nil, nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	return s.queryToRows(db, query)
}

// ExecuteQuery 鎵ц鏌ヨ
func (s *DatabaseService) ExecuteQuery(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, query string) ([][]string, []string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return nil, nil, err
	}

	return s.queryToRows(db, query)
}

// GetDatabaseSize 鑾峰彇鏁版嵁搴撳ぇ灏?func (s *DatabaseService) GetDatabaseSize(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) (string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return "", err
	}

	var query string
	switch dbType {
	case "mysql":
		query = "SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) FROM information_schema.tables WHERE table_schema = ?"
	case "postgresql":
		query = "SELECT pg_size_pretty(pg_database_size($1))"
	default:
		return "", fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	var size string
	err = db.QueryRow(query, dbDatabase).Scan(&size)
	if err != nil {
		return "", err
	}
	return size, nil
}

// GetTableStatus 鑾峰彇琛ㄧ姸鎬?func (s *DatabaseService) GetTableStatus(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) ([][]string, []string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return nil, nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = fmt.Sprintf("SHOW TABLE STATUS FROM `%s`", dbDatabase)
	case "postgresql":
		query = fmt.Sprintf(`
			SELECT relname AS table_name, n_live_tup AS row_count, pg_size_pretty(pg_total_relation_size(relid)) AS total_size
			FROM pg_stat_user_tables
			WHERE schemaname = '%s'
			ORDER BY n_live_tup DESC`, dbDatabase)
	default:
		return nil, nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	return s.queryToRows(db, query)
}

// TruncateTable 娓呯┖琛?func (s *DatabaseService) TruncateTable(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`", tableName))
	return err
}

// DropTable 鍒犻櫎琛?func (s *DatabaseService) DropTable(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase, tableName string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, dbDatabase)
	if err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName))
	return err
}

// CreateDatabase 鍒涘缓鏁版嵁搴?func (s *DatabaseService) CreateDatabase(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", dbDatabase))
	return err
}

// DropDatabase 鍒犻櫎鏁版嵁搴?func (s *DatabaseService) DropDatabase(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, dbDatabase string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE `%s`", dbDatabase))
	return err
}

// DetectDatabases 鑷姩妫€娴嬫湇鍔″櫒涓婂彲鐢ㄧ殑鏁版嵁搴?func (s *DatabaseService) DetectDatabases(sshConnID string) []map[string]string {
	client, err := s.sshSvc.GetClient(sshConnID)
	if err != nil || !client.IsConnected() {
		return nil
	}

	var results []map[string]string

	// 妫€娴?MySQL
	if out, err := client.ExecuteCommand("mysql --version 2>&1 || which mysql 2>&1"); err == nil && out != nil {
		version := strings.TrimSpace(out.Stdout)
		if version != "" && !strings.Contains(version, "not found") {
			results = append(results, map[string]string{
				"type": "mysql", "name": "MySQL", "port": "3306", "version": version,
			})
		}
	}

	// 妫€娴?PostgreSQL
	if out, err := client.ExecuteCommand("psql --version 2>&1 || which psql 2>&1"); err == nil && out != nil {
		version := strings.TrimSpace(out.Stdout)
		if version != "" && !strings.Contains(version, "not found") {
			results = append(results, map[string]string{
				"type": "postgresql", "name": "PostgreSQL", "port": "5432", "version": version,
			})
		}
	}

	// 妫€娴?Redis
	if out, err := client.ExecuteCommand("redis-cli --version 2>&1 || which redis-cli 2>&1"); err == nil && out != nil {
		version := strings.TrimSpace(out.Stdout)
		if version != "" && !strings.Contains(version, "not found") {
			results = append(results, map[string]string{
				"type": "redis", "name": "Redis", "port": "6379", "version": version,
			})
		}
	}

	// 妫€娴嬫鍦ㄨ繍琛岀殑鏈嶅姟绔彛
	portMap := map[string]string{
		"3306": "mysql", "5432": "postgresql", "6379": "redis", "27017": "mongodb",
	}
	out, _ := client.ExecuteCommand("ss -tlnp 2>/dev/null | grep -oE ':(3306|5432|6379|27017)' || netstat -tlnp 2>/dev/null | grep -oE ':(3306|5432|6379|27017)' || true")
	for _, line := range strings.Split(out.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		port := strings.TrimPrefix(line, ":")
		if dbType, ok := portMap[port]; ok {
			for i, r := range results {
				if r["type"] == dbType {
					results[i]["status"] = "running"
				}
			}
		}
	}

	return results
}

// queryToRows 鎵ц鏌ヨ骞惰繑鍥炵粨鏋勫寲缁撴灉
func (s *DatabaseService) queryToRows(db *sql.DB, query string) ([][]string, []string, error) {
	rows, err := db.Query(query)
	if err != nil {
		log.Error("[DB] queryToRows 鏌ヨ澶辫触", "error", err, "query", query)
		return nil, nil, fmt.Errorf("鏌ヨ澶辫触: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Error("[DB] queryToRows 鑾峰彇鍒楀悕澶辫触", "error", err)
		return nil, nil, err
	}

	log.Info("[DB] queryToRows 鍒楀悕", "columns", columns)

	var result [][]string
	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Error("[DB] queryToRows Scan 澶辫触", "error", err, "row", rowCount)
			continue
		}

		row := make([]string, len(columns))
		for i, v := range values {
			if v == nil {
				row[i] = "NULL"
			} else {
				switch val := v.(type) {
				case []byte:
					row[i] = string(val)
				case time.Time:
					row[i] = val.Format("2006-01-02 15:04:05")
				default:
					row[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		result = append(result, row)
		rowCount++
	}

	log.Info("[DB] queryToRows 瀹屾垚", "rows", rowCount, "query", query)
	return result, columns, nil
}

// GetProcessList 鑾峰彇鏁版嵁搴撹繘绋嬪垪琛?func (s *DatabaseService) GetProcessList(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword string) ([][]string, []string, error) {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return nil, nil, err
	}

	var query string
	switch dbType {
	case "mysql":
		query = "SHOW PROCESSLIST"
	case "postgresql":
		query = "SELECT pid, usename, datname, state, query, query_start FROM pg_stat_activity ORDER BY query_start DESC LIMIT 50"
	default:
		return nil, nil, fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	return s.queryToRows(db, query)
}

// KillQuery 缁堟鏌ヨ
func (s *DatabaseService) KillQuery(sshConnID, dbType, dbHost string, dbPort int, dbUser, dbPassword, queryID string) error {
	db, err := s.openDB(sshConnID, dbType, dbHost, dbPort, dbUser, dbPassword, "")
	if err != nil {
		return err
	}

	var query string
	switch dbType {
	case "mysql":
		query = fmt.Sprintf("KILL %s", queryID)
	case "postgresql":
		query = fmt.Sprintf("SELECT pg_terminate_backend(%s)", queryID)
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑鏁版嵁搴撶被鍨? %s", dbType)
	}

	_, err = db.Exec(query)
	return err
}

