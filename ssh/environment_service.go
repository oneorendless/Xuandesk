package ssh

import (
	"fmt"
	"strings"
	"sync"
)

// EnvInfo 妫€娴嬪埌鐨勭幆澧冧俊鎭?type EnvInfo struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Path        string   `json:"path"`              // 涓昏矾寰勶紙绗竴涓壘鍒扮殑锛?	Paths       []string `json:"paths,omitempty"`   // 鎵€鏈夋壘鍒扮殑璺緞
	Status      string   `json:"status"`            // installed / not_found
}

// envRule 鍗曚釜鐜鐨勬娴嬭鍒?type envRule struct {
	Name        string
	Category    string
	Description string
	// 妫€娴嬪懡浠わ細杈撳嚭璺緞锛岀┖琛ㄧず鏈壘鍒?	DetectCmd string
	// 鐗堟湰鍛戒护锛氳緭鍑虹増鏈彿
	VersionCmd string
}

// findBin 鐢熸垚妫€娴嬪懡浠わ細渚濇妫€鏌?which 鈫?甯歌璺緞
// paths 鏄宸ュ叿鍙兘瀹夎鐨勯澶栬矾寰勫垪琛?func findBin(bin string, paths ...string) string {
	cmds := []string{
		fmt.Sprintf("which %s 2>/dev/null", bin),
		fmt.Sprintf("command -v %s 2>/dev/null", bin),
	}
	for _, p := range paths {
		cmds = append(cmds, fmt.Sprintf("test -f %s && echo %s", p, p))
	}
	return strings.Join(cmds, " || ")
}

var envRules = []envRule{
	// ========== 缂栫▼璇█ ==========
	{Name: "Go", Category: "缂栫▼璇█", Description: "Google 寮€鍙戠殑闈欐€佺被鍨嬬紪璇戣瑷€",
		DetectCmd:  findBin("go", "/usr/local/go/bin/go", "/snap/bin/go", "$HOME/go/bin/go"),
		VersionCmd: "go version 2>/dev/null | awk '{print $3}' | sed 's/go//'"},
	{Name: "PHP", Category: "缂栫▼璇█", Description: "鏈嶅姟绔剼鏈瑷€锛屽箍娉涚敤浜?Web 寮€鍙?,
		DetectCmd:  findBin("php", "/usr/bin/php", "/usr/local/bin/php", "/usr/sbin/php", "/opt/*/bin/php"),
		VersionCmd: "php -v 2>/dev/null | head -1 | awk '{print $2}'"},
	{Name: "Python", Category: "缂栫▼璇█", Description: "閫氱敤楂樼骇缂栫▼璇█锛屽箍娉涚敤浜?AI/鏁版嵁绉戝",
		DetectCmd:  findBin("python3", "python", "/usr/bin/python3", "/usr/bin/python", "/usr/local/bin/python3", "/usr/local/bin/python"),
		VersionCmd: "python3 --version 2>/dev/null | awk '{print $2}' || python --version 2>/dev/null | awk '{print $2}'"},
	{Name: "Java", Category: "缂栫▼璇█", Description: "璺ㄥ钩鍙伴潰鍚戝璞¤瑷€锛屼紒涓氱骇搴旂敤棣栭€?,
		DetectCmd:  findBin("java", "/usr/bin/java", "/usr/local/bin/java", "/usr/lib/jvm/*/bin/java", "/opt/java/*/bin/java"),
		VersionCmd: "java -version 2>&1 | head -1 | awk -F'\"' '{print $2}'"},
	{Name: "Ruby", Category: "缂栫▼璇█", Description: "鍔ㄦ€佽剼鏈瑷€锛屽父鐢ㄤ簬 Web 寮€鍙戝拰鑷姩鍖?,
		DetectCmd:  findBin("ruby", "/usr/bin/ruby", "/usr/local/bin/ruby", "$HOME/.rbenv/shims/ruby", "$HOME/.rvm/rubies/*/bin/ruby"),
		VersionCmd: "ruby -v 2>/dev/null | awk '{print $2}'"},
	{Name: "Rust", Category: "缂栫▼璇█", Description: "绯荤粺绾х紪绋嬭瑷€锛屾敞閲嶅畨鍏ㄤ笌鎬ц兘",
		DetectCmd:  findBin("rustc", "/usr/bin/rustc", "$HOME/.cargo/bin/rustc", "/snap/bin/rustc"),
		VersionCmd: "rustc --version 2>/dev/null | awk '{print $2}'"},
	{Name: ".NET", Category: "缂栫▼璇█", Description: "寰蒋璺ㄥ钩鍙板紑鍙戞鏋?,
		DetectCmd:  findBin("dotnet", "/usr/bin/dotnet", "/usr/local/bin/dotnet", "/usr/share/dotnet/dotnet", "/snap/bin/dotnet"),
		VersionCmd: "dotnet --version 2>/dev/null"},
	{Name: "Perl", Category: "缂栫▼璇█", Description: "鏂囨湰澶勭悊鍜岀郴缁熺鐞嗚剼鏈瑷€",
		DetectCmd:  findBin("perl", "/usr/bin/perl", "/usr/local/bin/perl"),
		VersionCmd: "perl -v 2>/dev/null | grep 'version=' | head -1 | sed 's/.*v\\(\\[^ ]*\\).*/\\1/'"},
	{Name: "Erlang", Category: "缂栫▼璇█", Description: "鍑芥暟寮忕紪绋嬭瑷€锛岄珮骞跺彂鍦烘櫙棣栭€?,
		DetectCmd:  findBin("erl", "/usr/bin/erl", "/usr/local/bin/erl", "/usr/lib/erlang/bin/erl"),
		VersionCmd: "erl -eval 'erlang:display(erlang:system_info(otp_release)), halt().' -noshell 2>/dev/null | tr -d '\"'"},
	{Name: "Lua", Category: "缂栫▼璇█", Description: "杞婚噺绾у祵鍏ュ紡鑴氭湰璇█",
		DetectCmd:  findBin("lua", "lua5.4", "lua5.3", "/usr/bin/lua", "/usr/local/bin/lua", "/usr/bin/lua5.4", "/usr/bin/lua5.3"),
		VersionCmd: "lua -v 2>&1 | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "Kotlin", Category: "缂栫▼璇█", Description: "JVM 骞冲彴鐜颁唬缂栫▼璇█",
		DetectCmd:  findBin("kotlin", "/usr/bin/kotlin", "/usr/local/bin/kotlin", "/snap/bin/kotlin", "/opt/kotlin/bin/kotlin"),
		VersionCmd: "kotlin -version 2>&1 | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},

	// ========== 杩愯鏃?==========
	{Name: "Node.js", Category: "杩愯鏃?, Description: "鍩轰簬 V8 鐨?JavaScript 杩愯鏃?,
		DetectCmd:  findBin("node", "/usr/bin/node", "/usr/local/bin/node", "/snap/bin/node", "$HOME/.nvm/versions/node/*/bin/node", "$HOME/.volta/bin/node"),
		VersionCmd: "node -v 2>/dev/null | sed 's/^v//'"},
	{Name: "Deno", Category: "杩愯鏃?, Description: "瀹夊叏鐨?JavaScript/TypeScript 杩愯鏃?,
		DetectCmd:  findBin("deno", "/usr/bin/deno", "$HOME/.deno/bin/deno", "/snap/bin/deno"),
		VersionCmd: "deno --version 2>/dev/null | head -1 | awk '{print $2}'"},
	{Name: "Bun", Category: "杩愯鏃?, Description: "楂樻€ц兘 JavaScript 杩愯鏃朵笌鍖呯鐞嗗櫒",
		DetectCmd:  findBin("bun", "/usr/bin/bun", "$HOME/.bun/bin/bun", "/snap/bin/bun"),
		VersionCmd: "bun --version 2>/dev/null"},

	// ========== 鍖呯鐞嗗櫒 ==========
	{Name: "npm", Category: "鍖呯鐞?, Description: "Node.js 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("npm", "/usr/bin/npm", "/usr/local/bin/npm", "$HOME/.nvm/versions/node/*/bin/npm"),
		VersionCmd: "npm -v 2>/dev/null"},
	{Name: "yarn", Category: "鍖呯鐞?, Description: "蹇€熷彲闈犵殑 JavaScript 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("yarn", "/usr/bin/yarn", "/usr/local/bin/yarn", "$HOME/.yarn/bin/yarn"),
		VersionCmd: "yarn -v 2>/dev/null"},
	{Name: "pnpm", Category: "鍖呯鐞?, Description: "楂樻€ц兘 Node.js 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("pnpm", "/usr/bin/pnpm", "/usr/local/bin/pnpm", "$HOME/.local/share/pnpm/pnpm"),
		VersionCmd: "pnpm -v 2>/dev/null"},
	{Name: "pip", Category: "鍖呯鐞?, Description: "Python 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("pip3", "pip", "/usr/bin/pip3", "/usr/bin/pip", "/usr/local/bin/pip3", "/usr/local/bin/pip"),
		VersionCmd: "pip3 -V 2>/dev/null | awk '{print $2}' || pip -V 2>/dev/null | awk '{print $2}'"},
	{Name: "Cargo", Category: "鍖呯鐞?, Description: "Rust 鍖呯鐞嗗櫒涓庢瀯寤哄伐鍏?,
		DetectCmd:  findBin("cargo", "/usr/bin/cargo", "$HOME/.cargo/bin/cargo"),
		VersionCmd: "cargo --version 2>/dev/null | awk '{print $2}'"},
	{Name: "Composer", Category: "鍖呯鐞?, Description: "PHP 渚濊禆绠＄悊宸ュ叿",
		DetectCmd:  findBin("composer", "/usr/bin/composer", "/usr/local/bin/composer", "$HOME/.composer/vendor/bin/composer"),
		VersionCmd: "composer --version 2>/dev/null | awk '{print $NF}'"},
	{Name: "gem", Category: "鍖呯鐞?, Description: "Ruby 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("gem", "/usr/bin/gem", "/usr/local/bin/gem"),
		VersionCmd: "gem --version 2>/dev/null"},
	{Name: "apt", Category: "鍖呯鐞?, Description: "Debian/Ubuntu 杞欢鍖呯鐞嗗櫒",
		DetectCmd:  findBin("apt", "/usr/bin/apt", "/usr/bin/apt-get"),
		VersionCmd: "apt --version 2>/dev/null | head -1 | awk '{print $2}' | tr -d '()'"},
	{Name: "yum/dnf", Category: "鍖呯鐞?, Description: "CentOS/RHEL 杞欢鍖呯鐞嗗櫒",
		DetectCmd:  findBin("yum", "dnf", "/usr/bin/yum", "/usr/bin/dnf"),
		VersionCmd: "yum --version 2>/dev/null | head -1 || dnf --version 2>/dev/null | head -1"},
	{Name: "apk", Category: "鍖呯鐞?, Description: "Alpine Linux 杞欢鍖呯鐞嗗櫒",
		DetectCmd:  findBin("apk", "/sbin/apk", "/usr/sbin/apk"),
		VersionCmd: "apk --version 2>/dev/null | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},

	// ========== 鏁版嵁搴?==========
	{Name: "MySQL", Category: "鏁版嵁搴?, Description: "娴佽鐨勫紑婧愬叧绯诲瀷鏁版嵁搴?,
		DetectCmd:  findBin("mysql", "/usr/bin/mysql", "/usr/local/bin/mysql", "/usr/sbin/mysqld", "/opt/mysql/*/bin/mysql"),
		VersionCmd: "mysql --version 2>/dev/null | awk '{print $NF}'"},
	{Name: "PostgreSQL", Category: "鏁版嵁搴?, Description: "鍔熻兘寮哄ぇ鐨勫紑婧愬璞″叧绯绘暟鎹簱",
		DetectCmd:  findBin("psql", "/usr/bin/psql", "/usr/local/pgsql/bin/psql", "/usr/lib/postgresql/*/bin/psql"),
		VersionCmd: "psql --version 2>/dev/null | awk '{print $NF}'"},
	{Name: "MariaDB", Category: "鏁版嵁搴?, Description: "MySQL 鐨勫紑婧愬垎鏀?,
		DetectCmd:  findBin("mariadb", "/usr/bin/mariadb", "/usr/local/bin/mariadb", "/usr/sbin/mariadbd"),
		VersionCmd: "mariadb --version 2>/dev/null | awk '{print $NF}'"},
	{Name: "Redis", Category: "鏁版嵁搴?, Description: "楂樻€ц兘鍐呭瓨閿€兼暟鎹簱",
		DetectCmd:  findBin("redis-server", "/usr/bin/redis-server", "/usr/local/bin/redis-server", "/usr/sbin/redis-server"),
		VersionCmd: "redis-server --version 2>/dev/null | awk '{print $3}' | cut -d= -f2"},
	{Name: "MongoDB", Category: "鏁版嵁搴?, Description: "闈㈠悜鏂囨。鐨?NoSQL 鏁版嵁搴?,
		DetectCmd:  findBin("mongod", "/usr/bin/mongod", "/usr/local/bin/mongod", "/usr/bin/mongos", "/opt/mongodb/bin/mongod"),
		VersionCmd: "mongod --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "SQLite", Category: "鏁版嵁搴?, Description: "杞婚噺绾у祵鍏ュ紡鍏崇郴鏁版嵁搴?,
		DetectCmd:  findBin("sqlite3", "/usr/bin/sqlite3", "/usr/local/bin/sqlite3"),
		VersionCmd: "sqlite3 --version 2>/dev/null | awk '{print $1}'"},

	// ========== Web 鏈嶅姟鍣?==========
	{Name: "Nginx", Category: "Web 鏈嶅姟鍣?, Description: "楂樻€ц兘 HTTP 鍜屽弽鍚戜唬鐞嗘湇鍔″櫒",
		DetectCmd:  findBin("nginx", "/usr/sbin/nginx", "/usr/local/nginx/sbin/nginx", "/usr/local/bin/nginx", "/opt/nginx/sbin/nginx"),
		VersionCmd: "nginx -v 2>&1 | awk -F/ '{print $2}'"},
	{Name: "Apache", Category: "Web 鏈嶅姟鍣?, Description: "涓栫晫浣跨敤鏈€骞挎硾鐨?Web 鏈嶅姟鍣?,
		DetectCmd:  findBin("httpd", "apache2", "/usr/sbin/httpd", "/usr/sbin/apache2", "/usr/local/apache2/bin/httpd"),
		VersionCmd: "httpd -v 2>/dev/null | sed 's/.*Apache\\/\\([0-9.]*\\).*/\\1/' || apache2 -v 2>/dev/null | sed 's/.*Apache\\/\\([0-9.]*\\).*/\\1/'"},
	{Name: "Caddy", Category: "Web 鏈嶅姟鍣?, Description: "鑷姩 HTTPS 鐨勭幇浠?Web 鏈嶅姟鍣?,
		DetectCmd:  findBin("caddy", "/usr/bin/caddy", "/usr/local/bin/caddy", "/snap/bin/caddy"),
		VersionCmd: "caddy version 2>/dev/null | awk '{print $1}' | sed 's/^v//'"},
	{Name: "Tomcat", Category: "Web 鏈嶅姟鍣?, Description: "Java Servlet 瀹瑰櫒",
		DetectCmd:  findBin("catalina.sh", "/usr/bin/catalina.sh", "/opt/tomcat/bin/catalina.sh", "/opt/*/bin/catalina.sh", "/usr/share/tomcat*/bin/catalina.sh"),
		VersionCmd: "catalina.sh version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},

	// ========== 瀹瑰櫒 & 缂栨帓 ==========
	{Name: "Docker", Category: "瀹瑰櫒", Description: "瀹瑰櫒鍖栧簲鐢ㄩ儴缃插钩鍙?,
		DetectCmd:  findBin("docker", "/usr/bin/docker", "/usr/local/bin/docker", "/snap/bin/docker"),
		VersionCmd: "docker --version 2>/dev/null | awk '{for(i=3;i<=NF;i++){gsub(/,/,\"\",$i); if($i ~ /^[0-9]/){print $i; exit}}}'"},
	{Name: "Podman", Category: "瀹瑰櫒", Description: "鏃犲畧鎶よ繘绋嬬殑瀹瑰櫒寮曟搸",
		DetectCmd:  findBin("podman", "/usr/bin/podman", "/usr/local/bin/podman"),
		VersionCmd: "podman --version 2>/dev/null | awk '{print $2}'"},
	{Name: "containerd", Category: "瀹瑰櫒", Description: "宸ヤ笟绾у鍣ㄨ繍琛屾椂",
		DetectCmd:  findBin("containerd", "/usr/bin/containerd", "/usr/local/bin/containerd", "/usr/sbin/containerd"),
		VersionCmd: "containerd --version 2>/dev/null | awk '{print $3}'"},
	{Name: "kubectl", Category: "瀹瑰櫒", Description: "Kubernetes 鍛戒护琛屽伐鍏?,
		DetectCmd:  findBin("kubectl", "/usr/bin/kubectl", "/usr/local/bin/kubectl", "/snap/bin/kubectl", "$HOME/.local/bin/kubectl"),
		VersionCmd: "kubectl version --client 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^v?[0-9]/){gsub(/^v/,\"\",$i); print $i; exit}}'"},
	{Name: "Helm", Category: "瀹瑰櫒", Description: "Kubernetes 鍖呯鐞嗗櫒",
		DetectCmd:  findBin("helm", "/usr/bin/helm", "/usr/local/bin/helm", "/snap/bin/helm"),
		VersionCmd: "helm version --short 2>/dev/null | sed 's/^v//'"},

	// ========== DevOps ==========
	{Name: "Terraform", Category: "DevOps", Description: "鍩虹璁炬柦鍗充唬鐮佸伐鍏?,
		DetectCmd:  findBin("terraform", "/usr/bin/terraform", "/usr/local/bin/terraform", "/snap/bin/terraform"),
		VersionCmd: "terraform version 2>/dev/null | head -1 | awk '{print $2}' | sed 's/^v//'"},
	{Name: "Ansible", Category: "DevOps", Description: "鑷姩鍖栬繍缁撮厤缃鐞嗗伐鍏?,
		DetectCmd:  findBin("ansible", "/usr/bin/ansible", "/usr/local/bin/ansible", "$HOME/.local/bin/ansible"),
		VersionCmd: "ansible --version 2>/dev/null | head -1 | awk '{print $2}'"},

	// ========== 鏋勫缓宸ュ叿 ==========
	{Name: "Make", Category: "鏋勫缓宸ュ叿", Description: "缁忓吀鏋勫缓鑷姩鍖栧伐鍏?,
		DetectCmd:  findBin("make", "/usr/bin/make", "/usr/local/bin/make"),
		VersionCmd: "make --version 2>/dev/null | head -1 | awk '{print $3}'"},
	{Name: "CMake", Category: "鏋勫缓宸ュ叿", Description: "璺ㄥ钩鍙版瀯寤虹郴缁?,
		DetectCmd:  findBin("cmake", "/usr/bin/cmake", "/usr/local/bin/cmake", "/snap/bin/cmake"),
		VersionCmd: "cmake --version 2>/dev/null | head -1 | awk '{print $3}'"},
	{Name: "GCC", Category: "鏋勫缓宸ュ叿", Description: "GNU 缂栬瘧鍣ㄥ浠?,
		DetectCmd:  findBin("gcc", "/usr/bin/gcc", "/usr/local/bin/gcc"),
		VersionCmd: "gcc --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "G++", Category: "鏋勫缓宸ュ叿", Description: "GNU C++ 缂栬瘧鍣?,
		DetectCmd:  findBin("g++", "/usr/bin/g++", "/usr/local/bin/g++"),
		VersionCmd: "g++ --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "Clang", Category: "鏋勫缓宸ュ叿", Description: "LLVM C/C++ 缂栬瘧鍣?,
		DetectCmd:  findBin("clang", "/usr/bin/clang", "/usr/local/bin/clang"),
		VersionCmd: "clang --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "Gradle", Category: "鏋勫缓宸ュ叿", Description: "鍩轰簬 Groovy/Kotlin 鐨勬瀯寤哄伐鍏?,
		DetectCmd:  findBin("gradle", "/usr/bin/gradle", "/usr/local/bin/gradle", "/opt/gradle/bin/gradle", "$HOME/.sdkman/candidates/gradle/*/bin/gradle"),
		VersionCmd: "gradle --version 2>/dev/null | grep 'Gradle' | awk '{print $2}'"},
	{Name: "Maven", Category: "鏋勫缓宸ュ叿", Description: "Java 椤圭洰绠＄悊鍜屾瀯寤哄伐鍏?,
		DetectCmd:  findBin("mvn", "/usr/bin/mvn", "/usr/local/bin/mvn", "/opt/maven/bin/mvn", "$HOME/.sdkman/candidates/maven/*/bin/mvn"),
		VersionCmd: "mvn --version 2>/dev/null | head -1 | awk '{print $3}'"},

	// ========== 鐗堟湰鎺у埗 ==========
	{Name: "Git", Category: "鐗堟湰鎺у埗", Description: "鍒嗗竷寮忕増鏈帶鍒剁郴缁?,
		DetectCmd:  findBin("git", "/usr/bin/git", "/usr/local/bin/git", "/snap/bin/git"),
		VersionCmd: "git --version 2>/dev/null | awk '{print $3}'"},
	{Name: "SVN", Category: "鐗堟湰鎺у埗", Description: "闆嗕腑寮忕増鏈帶鍒剁郴缁?,
		DetectCmd:  findBin("svn", "/usr/bin/svn", "/usr/local/bin/svn"),
		VersionCmd: "svn --version --quiet 2>/dev/null"},

	// ========== 缃戠粶宸ュ叿 ==========
	{Name: "curl", Category: "缃戠粶宸ュ叿", Description: "鍛戒护琛屾暟鎹紶杈撳伐鍏?,
		DetectCmd:  findBin("curl", "/usr/bin/curl", "/usr/local/bin/curl", "/snap/bin/curl"),
		VersionCmd: "curl --version 2>/dev/null | head -1 | awk '{print $2}'"},
	{Name: "wget", Category: "缃戠粶宸ュ叿", Description: "缃戠粶涓嬭浇宸ュ叿",
		DetectCmd:  findBin("wget", "/usr/bin/wget", "/usr/local/bin/wget"),
		VersionCmd: "wget --version 2>/dev/null | head -1 | awk '{print $3}'"},
	{Name: "OpenSSH", Category: "缃戠粶宸ュ叿", Description: "瀹夊叏杩滅▼杩炴帴鍗忚濂椾欢",
		DetectCmd:  findBin("ssh", "/usr/bin/ssh", "/usr/local/bin/ssh"),
		VersionCmd: "ssh -V 2>&1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "rsync", Category: "缃戠粶宸ュ叿", Description: "蹇€熷閲忔枃浠跺悓姝ュ伐鍏?,
		DetectCmd:  findBin("rsync", "/usr/bin/rsync", "/usr/local/bin/rsync"),
		VersionCmd: "rsync --version 2>/dev/null | head -1 | awk '{print $3}'"},

	// ========== 瀹夊叏宸ュ叿 ==========
	{Name: "OpenSSL", Category: "瀹夊叏宸ュ叿", Description: "TLS/SSL 鍔犲瘑搴撲笌宸ュ叿",
		DetectCmd:  findBin("openssl", "/usr/bin/openssl", "/usr/local/bin/openssl", "/usr/sbin/openssl"),
		VersionCmd: "openssl version 2>/dev/null | awk '{print $2}'"},
	{Name: "GPG", Category: "瀹夊叏宸ュ叿", Description: "GNU 闅愮淇濇姢鍔犲瘑宸ュ叿",
		DetectCmd:  findBin("gpg", "gpg2", "/usr/bin/gpg", "/usr/bin/gpg2", "/usr/local/bin/gpg"),
		VersionCmd: "gpg --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "Fail2ban", Category: "瀹夊叏宸ュ叿", Description: "鍏ヤ镜闃插尽妗嗘灦",
		DetectCmd:  findBin("fail2ban-client", "/usr/bin/fail2ban-client", "/usr/local/bin/fail2ban-client"),
		VersionCmd: "fail2ban-client --version 2>/dev/null | head -1 | awk '{print $2}'"},

	// ========== 绯荤粺宸ュ叿 ==========
	{Name: "tmux", Category: "绯荤粺宸ュ叿", Description: "缁堢澶嶇敤鍣?,
		DetectCmd:  findBin("tmux", "/usr/bin/tmux", "/usr/local/bin/tmux", "/snap/bin/tmux"),
		VersionCmd: "tmux -V 2>/dev/null | awk '{print $2}'"},
	{Name: "screen", Category: "绯荤粺宸ュ叿", Description: "缁堢浼氳瘽绠＄悊鍣?,
		DetectCmd:  findBin("screen", "/usr/bin/screen", "/usr/local/bin/screen"),
		VersionCmd: "screen -v 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^[0-9]/){print $i; exit}}'"},
	{Name: "htop", Category: "绯荤粺宸ュ叿", Description: "浜や簰寮忚繘绋嬫煡鐪嬪櫒",
		DetectCmd:  findBin("htop", "/usr/bin/htop", "/usr/local/bin/htop", "/snap/bin/htop"),
		VersionCmd: "htop --version 2>/dev/null | head -1 | awk '{print $2}'"},
	{Name: "jq", Category: "绯荤粺宸ュ叿", Description: "杞婚噺绾у懡浠よ JSON 澶勭悊鍣?,
		DetectCmd:  findBin("jq", "/usr/bin/jq", "/usr/local/bin/jq", "/snap/bin/jq"),
		VersionCmd: "jq --version 2>/dev/null | sed 's/^jq-//'"},
	{Name: "Vim", Category: "绯荤粺宸ュ叿", Description: "楂樺害鍙厤缃殑鏂囨湰缂栬緫鍣?,
		DetectCmd:  findBin("vim", "vi", "/usr/bin/vim", "/usr/bin/vi", "/usr/local/bin/vim"),
		VersionCmd: "vim --version 2>/dev/null | head -1 | awk '{for(i=2;i<=NF;i++) if($i ~ /^V?[0-9]/){gsub(/^V/,\"\",$i); print $i; exit}}'"},
	{Name: "Neovim", Category: "绯荤粺宸ュ叿", Description: "鐜颁唬鍖?Vim 鏂囨湰缂栬緫鍣?,
		DetectCmd:  findBin("nvim", "/usr/bin/nvim", "/usr/local/bin/nvim", "/snap/bin/nvim"),
		VersionCmd: "nvim --version 2>/dev/null | head -1 | awk '{print $2}'"},
}

// GetSystemEnvironments 骞跺彂妫€娴嬭繙绋嬫湇鍔″櫒涓婄殑鎵€鏈夌幆澧?func (c *SSHClient) GetSystemEnvironments() ([]EnvInfo, error) {
	if c.isConnected.Load() == 0 {
		return nil, fmt.Errorf("SSH 鏈繛鎺?)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]EnvInfo, 0, len(envRules))

	sem := make(chan struct{}, 12)

	for _, rule := range envRules {
		wg.Add(1)
		go func(r envRule) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			env := c.detectOne(r)
			mu.Lock()
			results = append(results, env)
			mu.Unlock()
		}(rule)
	}

	wg.Wait()

	var installed, notFound []EnvInfo
	for _, e := range results {
		if e.Status == "installed" {
			installed = append(installed, e)
		} else {
			notFound = append(notFound, e)
		}
	}

	return append(installed, notFound...), nil
}

// detectOne 妫€娴嬪崟涓幆澧?func (c *SSHClient) detectOne(r envRule) EnvInfo {
	env := EnvInfo{
		Name:        r.Name,
		Category:    r.Category,
		Description: r.Description,
		Status:      "not_found",
	}

	// 妫€娴嬭矾寰勶紙DetectCmd 宸插寘鍚?which + 甯歌璺緞锛?	pathResult, err := c.ExecuteCommand(r.DetectCmd)
	if err == nil && pathResult.Success {
		// 鏀堕泦鎵€鏈夋壘鍒扮殑璺緞锛堝幓閲嶏級
		paths := uniqueLines(pathResult.Stdout)
		if len(paths) > 0 {
			env.Path = paths[0]
			if len(paths) > 1 {
				env.Paths = paths
			}
			env.Status = "installed"
		}
	}

	// 濡傛灉 DetectCmd 澶辫触锛屽皾璇曞寘绠＄悊鍣ㄦ煡璇?	if env.Status == "not_found" {
		binName := extractBinaryName(r.DetectCmd)
		pkgCmd := fmt.Sprintf(
			`(dpkg -L %s 2>/dev/null | grep '/bin/%s$' | head -1) || (rpm -ql %s 2>/dev/null | grep '/bin/%s$' | head -1)`,
			r.Name, binName, r.Name, binName,
		)
		pkgResult, err := c.ExecuteCommand(pkgCmd)
		if err == nil && pkgResult.Success {
			path := strings.TrimSpace(pkgResult.Stdout)
			if path != "" {
				env.Path = path
				env.Status = "installed"
			}
		}
		// 濡傛灉涓婇潰娌℃壘鍒板叿浣撹矾寰勶紝鏌ヨ鐗堟湰淇℃伅
		if env.Status == "not_found" {
			verCmd := fmt.Sprintf(
				`(dpkg -s %s 2>/dev/null | grep '^Version:' | awk '{print $2}') || (rpm -q %s 2>/dev/null | sed 's/^.*-//')`,
				r.Name, r.Name,
			)
			verResult, err := c.ExecuteCommand(verCmd)
			if err == nil && verResult.Success {
				ver := strings.TrimSpace(verResult.Stdout)
				if ver != "" {
					env.Version = ver
					env.Path = "(via " + r.Name + " package)"
					env.Status = "installed"
				}
			}
		}
	}

	// 鑾峰彇鐗堟湰
	if env.Status == "installed" && r.VersionCmd != "" && env.Version == "" {
		verResult, err := c.ExecuteCommand(r.VersionCmd)
		if err == nil && verResult.Success {
			env.Version = strings.TrimSpace(verResult.Stdout)
		}
	}

	return env
}

// uniqueLines 浠庡琛岃緭鍑轰腑鎻愬彇鎵€鏈夐潪绌恒€佸幓閲嶇殑琛?func uniqueLines(output string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !seen[line] {
			seen[line] = true
			result = append(result, line)
		}
	}
	return result
}

// extractBinaryName 浠?DetectCmd 涓彁鍙栬妫€娴嬬殑浜岃繘鍒跺悕绉?func extractBinaryName(cmd string) string {
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if f == "which" || f == "command" || f == "type" {
			for j := i + 1; j < len(fields); j++ {
				next := fields[j]
				if next == "-v" || next == "-p" || next == "-n" || next == "2>/dev/null" || next == "||" || next == "&&" {
					continue
				}
				if strings.HasPrefix(next, "-") {
					continue
				}
				return next
			}
		}
	}
	for _, f := range fields {
		if !strings.HasPrefix(f, "-") && f != "2>/dev/null" && f != "||" && f != "&&" && f != "|" {
			return f
		}
	}
	return ""
}

