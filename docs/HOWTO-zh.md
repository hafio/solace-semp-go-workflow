# SEMP Workflow Automation — 操作指南

> 適用平台：Solace PubSub+（SEMP v2）
> 動作模組由 `specs/` 中的 SEMP 10.26.0.8755 結構描述（Schema）產生

---

## 目錄

1. [簡介](#1-簡介)
2. [系統需求](#2-系統需求)
3. [開始使用](#3-開始使用)
4. [專案結構](#4-專案結構)
5. [設定檔](#5-設定檔)
6. [工作流程範本](#6-工作流程範本)
7. [CLI 指令](#7-cli-指令)
8. [內建範本說明](#8-內建範本說明)
9. [可用模組](#9-可用模組)
10. [常見情境範例](#10-常見情境範例)
11. [故障排除](#11-故障排除)

---

## 1. 簡介

SEMP Workflow Automation 是一套類似 Ansible Playbook 的命令列工具，專為 Solace PubSub+ 訊息代理人（Message Broker）設計。透過宣告式 YAML 設定檔，您可以批次建立、刪除或更新佇列（Queue）、REST 遞送點（RDP），以及 SEMP v2 設定 API 所提供的任何物件，無需手動操作管理介面。

**主要特色：**

- **完整涵蓋 SEMP v2 設定 API**：共 225 個動作、86 種物件家族，全部由 Broker 自身的結構描述產生，而非人工逐一撰寫。除訊息 VPN 層級的物件外，也包含 Broker 層級的物件：訊息 VPN 本身、DMR 叢集與連線、OAuth 設定檔、憑證授權單位（CA）、虛擬主機名稱，以及 Broker 設定物件。
- **冪等操作（Idempotent）**：每個動作在執行前先檢查目前狀態，因此可安全地重複執行。`add` 若物件已存在則跳過；`update` 會比對 Broker 上的現有值，若已符合預期則回報未變更。
- **在送出前於本機驗證**：屬性名稱拼錯、值不在允許範圍內、缺少必填屬性，或指定了建立後不可變更的屬性，都會在本機回報並指出對應的工作流程與任務，而不是送到 Broker 後才被拒絕。
- **Jinja2 變數渲染**：支援動態命名與跨變數參考。
- **試運行模式（Dry-run）**：可預覽變更內容而不實際執行。

**若您曾使用舊版，以下兩項行為已變更：**

- `update` 在物件已符合指定值時會回報 `skipped`，而非一律回報 `changed`。Broker 不會回傳的屬性（例如密碼）無法比對，因此每次都會重新送出。
- 動作的參數會依該物件的完整屬性集合檢查。若參數不屬於該物件的屬性，視為錯誤，不再直接轉送至 Broker。

---

## 2. 系統需求

| 需求項目 | 版本要求 |
|---|---|
| Go | 1.26 以上（僅在從原始碼建置時需要；預先建置的二進位檔為獨立執行檔） |
| Solace PubSub+ Broker | 支援 SEMP v2 API 的版本 |
| 網路連線 | 可存取 Broker 的 SEMP 管理埠（預設 `8080` / `1943`） |

---

## 3. 開始使用

`semp-workflow` 是單一獨立執行的二進位檔 —— Go 執行環境與所有相依套件皆已編譯進去，工作流程範本則透過 `//go:embed` 內嵌，因此執行時無需任何直譯器或套件安裝。

### 安裝預先建置的二進位檔（建議）

每個 [GitHub 發行版本](https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases) 都會為各支援平台附上一個二進位檔 —— `semp-workflow-{linux,darwin,windows}-{amd64,arm64}`（Windows 會多出 `.exe`）—— 以及一個 `SHA256SUMS.txt` 檔案。下載對應你平台的檔案、驗證校驗碼，再放入 `PATH` 即可：

```bash
base=https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases/latest/download
curl -LO "$base/semp-workflow-linux-amd64"     # 依你的 OS/arch 調整
curl -LO "$base/SHA256SUMS.txt"
sha256sum --ignore-missing -c SHA256SUMS.txt    # → semp-workflow-linux-amd64: OK
install -m 0755 semp-workflow-linux-amd64 /usr/local/bin/semp-workflow
```

發行版二進位檔會以發行標籤作為版本號：執行 `semp-workflow --version` 會顯示例如 `semp-workflow, version v0.4.0`。

### 從原始碼建置（Go 1.26 以上）

```bash
./scripts/dev.sh build      # 於儲存庫根目錄執行；Windows：.\scripts\dev.ps1 build
# → 產生 dist/semp-workflow-<os>-<arch>[.exe]，供主機使用（例如 dist/semp-workflow-linux-amd64）
```

或直接使用 Go 工具鏈建置／執行：

```bash
go build ./cmd/semp-workflow          # 或：go run ./cmd/semp-workflow
```

若要瞭解二進位檔大小的組成，可執行僅供檢視的 `size` 任務。它會重新建置二進位檔，
並透過 [go-size-analyzer](https://github.com/Zxilly/go-size-analyzer) 依套件／模組
列出大小明細：

```bash
./scripts/dev.sh size       # Windows：.\scripts\dev.ps1 size
# 首次執行需連網以下載工具（不會全域安裝任何東西）
# 無工具時的備援：go tool nm -size -sort=size dist/semp-workflow-<os>-<arch>
```

接著執行：

```bash
semp-workflow --help
```

初次使用時，可將內建範本匯出至本機目錄供自訂修改：

```bash
semp-workflow init --output-dir templates
```

---

## 4. 專案結構

```
專案目錄/
├── semp-workflow(.exe)       # 工具本體（單一獨立執行的二進位檔）
├── config.yaml               # 主設定檔（連線資訊 + 工作流程清單）
└── templates/                # 選用 —— 僅在執行 `init` 或覆寫內嵌範本時才需要
```

- **`semp-workflow`**：單一獨立執行的二進位檔（Windows 為 `.exe`）；工作流程範本透過 `//go:embed` 內嵌
- **`config.yaml`**：定義 SEMP 連線、全域變數（`global_vars`）及要執行的工作流程清單
- **`templates/`**（選用）：內建範本已內嵌於二進位檔中；執行 `init` 可將其匯出。每個 YAML 檔包含一或多個具名範本，每個範本定義輸入變數與動作序列

---

## 5. 設定檔

`config.yaml` 分為三個主要區塊：

### 5.1 SEMP 連線設定

```yaml
semp:
  host: "https://broker.example.com:1943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false   # 自簽憑證環境設為 false
  timeout: 30         # 秒
```

**說明：** `host` 必須是完整的 SEMP v2 管理 URL，包含通訊協定與埠號。工具使用 HTTP Basic Auth 進行認證。所有操作均限定在指定的 `msg_vpn` 範圍內。

| 欄位 | 必填 | 預設值 | 說明 |
|---|---|---|---|
| `host` | 是 | -- | 完整 URL 含埠號，如 `https://broker:8943` |
| `username` | 是 | -- | SEMP 管理員帳號 |
| `password` | 是 | -- | SEMP 管理員密碼 |
| `msg_vpn` | 是 | -- | 要操作的 Message VPN |
| `verify_ssl` | 否 | `false` | 驗證 TLS 憑證 |
| `timeout` | 否 | `30` | HTTP 請求逾時（秒） |

### 5.2 全域變數（global_vars）

全域變數可在所有工作流程的輸入欄位中透過 `{{ global_vars.變數名稱 }}` 引用，方便統一管理重複設定值。

```yaml
global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_queue_owner: "svc-app-client"
  default_rc_remote_host: "my-backend.example.com"
```

**說明：** 全域變數在 Jinja2 第一輪渲染時解析。它們也可以包含 `{{ inputs.X }}` 引用，這些會在第二輪渲染時解析。這使其非常適合定義多個工作流程共用的命名慣例。

### 5.3 工作流程清單（workflows）

每個工作流程條目指定要套用的範本以及輸入變數：

```yaml
workflows:
  - template: "app-outbound.new-seq"    # 格式：檔名.範本名稱
    inputs:
      domain: "CENTRAL"                       # 必填輸入
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-EVENT"
      # 選填輸入（移除 # 號可覆蓋範本預設值）：
      #service_queue_owner: "{{ global_vars.default_queue_owner }}"
```

> **注意**：範本參照格式為 `檔名.範本名稱`，例如 `app-outbound.new-seq` 代表 `app-outbound.yaml` 中名為 `new-seq` 的範本。

**說明：** 工作流程按照清單中的順序由上而下執行。每個工作流程條目獨立運作，您可以自由混合不同範本與輸入值。註解的輸入表示可用的選填覆蓋項；取消註解即可變更範本預設值。

### 5.4 完整設定檔範例

```yaml
semp:
  host: "https://broker-host:8943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false
  timeout: 30

global_vars:
  topic_prefix:    "myapp/events"
  default_owner:   "my-client"
  queue_ttl:       1296000

templates_dir: "templates"

workflows:
  - template: "my-queues.create"
    inputs:
      queue_name: "MY-QUEUE"
      sub_topic:  "{{ global_vars.topic_prefix }}/>"

  - template: "my-queues.create"
    inputs:
      queue_name: "MY-OTHER-QUEUE"
      sub_topic:  "other/topic/>"
```

---

## 6. 工作流程範本

範本檔案定義可重複使用的工作流程，結構如下：

```yaml
workflow-templates:
  - name: "my-template"

    inputs:
      required:           # 必填輸入（未提供時報錯）
      - domain
      - system

      optional:           # 選填輸入（提供預設值）
        queue_name: "Q-{{ inputs.domain }}-{{ inputs.system }}"
        queue_owner: ""

    actions:
    - name: "建立佇列"
      module: "msg_vpn_queue.add"
      args:
        queue_name: "{{ inputs.queue_name }}"
        owner: "{{ inputs.queue_owner }}"
```

**說明：** 範本定義了一個契約：哪些輸入是必填的、哪些是選填的（含預設值），以及要執行哪些動作。動作按順序執行，每個動作對應一個內建模組（如 `msg_vpn_queue.add` 或 `msg_vpn_rest_delivery_point.delete`）。`args` 區塊支援完整的 Jinja2 運算式，因此您可以從輸入和全域變數組合動態值。

### 輸入結構

| 鍵 | 格式 | 說明 |
|---|---|---|
| `required` | 字串清單 | 呼叫者必須提供的輸入 |
| `optional` | `名稱: 預設值` 對應表 | 選填輸入；預設值可以是文字值或 Jinja2 運算式 |

預設值為 `null` 的選填輸入會被納入結構但沒有預設值——若呼叫者未提供，將從解析後的上下文中省略。

### 變數渲染規則

| 語法 | 用途 |
|---|---|
| `{{ inputs.變數名稱 }}` | 引用輸入變數 |
| `{{ global_vars.變數名稱 }}` | 引用全域變數（僅在預設值中使用） |
| `{{ inputs.a }}-{{ inputs.b }}` | 組合多個變數 |

### 兩階段渲染說明

範本透過兩個階段進行渲染：

1. **第一階段：** `global_vars` 上下文可用。如 `"{{ global_vars.topic_prefix }}"` 的預設值會被解析。
2. **第二階段：** 完整的 `inputs` 字典可用。如 `"DMQ/{{ inputs.queue_name }}"` 的預設值會被解析。

此兩階段設計允許全域變數定義引用輸入的命名模式（例如 `queue_name_tpl: "Q-{{ inputs.domain }}-{{ inputs.system }}"`）。全域變數在第一階段展開為原始 Jinja 字串，然後在第二階段解析其中的 `{{ inputs.X }}` 引用。

### YAML 錨點（Anchor）支援

範本支援 YAML 錨點與別名，可共用輸入定義或動作清單：

```yaml
workflow-templates:
  - name: "create-seq"
    inputs:
      required: &required-inputs
        - queue_name
        - sub_topic
      optional: &optional-inputs
        access_type: exclusive
    actions: &create-actions
      - name: "Create Queue"
        module: "msg_vpn_queue.add"
        args:
          queue_name: "{{ inputs.queue_name }}"

  - name: "create-non-seq"
    inputs:
      required: *required-inputs
      optional:
        <<: *optional-inputs
        access_type: non-exclusive  # 覆蓋一個欄位
    actions: *create-actions
```

---

## 7. CLI 指令

### 頂層說明

```
$ semp-workflow --help

SEMP Workflow Automation - Ansible-like playbooks for Solace SEMP.

Usage:
  semp-workflow [command]

Available Commands:
  help          Help about any command
  init          Copy bundled workflow templates to a local directory.
  list-modules  List all available action modules.
  run           Execute workflows defined in a config file.
  validate      Validate config and templates without executing.

Flags:
  -h, --help      help for semp-workflow
      --version   version for semp-workflow

Use "semp-workflow [command] --help" for more information about a command.
```

---

### 7.1 執行工作流程

```
$ semp-workflow run --help

Execute workflows defined in a config file.

Usage:
  semp-workflow run [flags]

Flags:
      --check                  Alias for --dry-run.
  -c, --config string          Path to config YAML file.
      --dry-run                Show what would be done without making changes.
  -f, --fail-fast              Stop execution on first failure.
  -h, --help                   help for run
  -t, --templates-dir string   Override the workflow templates directory.
  -v, --verbose                Enable verbose/debug logging.
```

注意：`-c/--config` 為必填 —— 省略時會回報 `required flag(s) "config" not set`（結束代碼 `2`）。`-v/--verbose` 為相容性保留選項，但 Go 模組不會輸出額外的除錯訊息。

**結束代碼：**

| 代碼 | 含義 |
|---|---|
| `0` | 所有工作流程成功完成，無失敗 |
| `1` | 一個或多個工作流程動作失敗 |
| `2` | 設定檔或範本錯誤（未執行任何動作） |
| `130` | 使用者中斷（`Ctrl+C`） |

**範例：**
```bash
# 試運行（不實際執行）
semp-workflow run -c config.yaml --dry-run

# 遇錯即停，並顯示詳細日誌
semp-workflow run -c config.yaml --fail-fast --verbose

# 使用自訂範本目錄
semp-workflow run -c config.yaml --templates-dir ./my-templates
```

---

### 7.2 驗證設定檔

```
$ semp-workflow validate --help

Validate config and templates without executing.

Usage:
  semp-workflow validate [flags]

Flags:
  -c, --config string          Path to config YAML file.
  -h, --help                   help for validate
  -t, --templates-dir string   Override the workflow templates directory.
```

**說明：** 完全在本機檢查整份設定，不連線至 Broker，因此可即時執行。檢查項目如下：

1. 設定檔與所有範本可正確解析；
2. `workflows` 清單中的每個 `template` 引用對應到實際範本；
3. 每個必填輸入皆已提供，且所有輸入皆可解析（無法解析或互相循環參照的 Jinja2 運算式會被指出）；
4. 每個動作所指定的模組皆已註冊；
5. 每個動作的參數對該模組皆為有效 —— 沒有未知屬性、值落在允許範圍內、必填屬性齊備、`update` 中未出現僅能於建立時設定的屬性、沒有超出長度上限的屬性，也沒有 Broker 不允許的屬性組合。

一次執行會列出所有問題，因此可一次修正整份設定，不需逐一試錯。

**範例：**
```bash
semp-workflow validate -c config.yaml
```

錯誤訊息會指出工作流程、任務與屬性：

```
ERROR: workflow 1 (app-inbound.new-seq), task 'Create queue' [msg_vpn_queue.add]: unknown argument: acessType (see docs/modules/msg_vpn_queue.md for the accepted attributes)
```

在對正式環境 Broker 執行 `run` 之前請先執行此指令。它不需任何成本，卻能攔截原本要等到工作流程執行到一半才會發現的錯誤。

---

### 7.3 列出所有可用模組

```
$ semp-workflow list-modules --help

List all available action modules.

Usage:
  semp-workflow list-modules [flags]

Flags:
  -h, --help                help for list-modules
  -o, --output string       Write the whole module reference to one Markdown file.
      --output-dir string   Write the module reference as an index plus one file per object family (e.g. docs/modules).
```

**說明：** 顯示所有已註冊的動作模組及其說明。

`--output-dir` 會以本專案所採用的形式輸出參考文件：一份列出所有物件家族的 `index.md`，加上每個家族一份頁面，其中包含各動作的參數表。建議使用此形式 —— 在完整涵蓋 SEMP 的情況下，單一文件會超過一萬行。若某個家族已不存在，其頁面會一併移除，因此目錄不會殘留新版結構描述已移除物件的頁面。

`--output` 仍可將全部內容寫入單一檔案，偶爾便於搜尋。

**範例：**
```bash
semp-workflow list-modules
semp-workflow list-modules --output-dir docs/modules
```

已納入版本控制的參考文件位於 [docs/modules/index.md](modules/index.md)。

---

### 7.4 匯出內建範本

```
$ semp-workflow init --help

Copy bundled workflow templates to a local directory.

Usage:
  semp-workflow init [flags]

Flags:
  -f, --force               Overwrite existing files.
  -h, --help                help for init
  -o, --output-dir string   Directory to copy bundled templates into. (default "templates")
```

**說明：** 範本透過 `//go:embed` 內嵌於二進位檔中。`init` 指令將其寫出至本機目錄以供自訂修改。已存在的檔案預設會跳過，除非使用 `--force`。

**範例：**
```bash
semp-workflow init
semp-workflow init --output-dir my-templates
semp-workflow init --output-dir templates --force
```

---

## 8. 內建範本說明

### app-outbound — APP 出站工作流程

訊息流向：Solace -> APP（Broker 接收訊息後轉送至下游）

| 範本 | 說明 |
|---|---|
| `app-outbound.new-seq` | 建立循序遞送佇列組合（Service Queue + Mirror Queue + DMQ + 訂閱） |
| `app-outbound.new-non-seq` | 建立並發遞送佇列組合（與 new-seq 相同結構，預設重遞送次數不同） |
| `app-outbound.delete` | 刪除出站佇列組合 |

**必填輸入：**

| 變數 | 說明 | 範例 |
|---|---|---|
| `domain` | 業務領域 | `CENTRAL` |
| `system` | 系統名稱 | `APPSYS` |
| `system_topic` | 主題識別碼 | `SITEB.ORDERS.ORDER-DETAIL` |

**建立的資源：** `new-seq` 和 `new-non-seq` 範本除了建立佇列和訂閱外，還會佈建 Client Profile、per-user ACL Profile、Client Username 及發佈主題例外。`new-non-seq` 變體僅在 `service_queue_max_redelivery` 上不同（5 vs 0）。`delete` 範本按反向相依順序移除佇列和 per-user 存取控制資源。

> 完整參數與動作參考請見 **[docs/template-app-outbound.md](template-app-outbound.md)**。

---

### app-inbound — APP 入站工作流程

訊息流向：APP -> Solace -> 後端 REST 服務

| 範本 | 說明 |
|---|---|
| `app-inbound.new-seq` | 建立循序入站流程（佇列組合 + RDP + REST Consumer + Queue Binding），訂閱主題格式：`domain/system/topic` |
| `app-inbound.new-non-seq` | 建立並發入站流程，訂閱主題格式：`topic_prefix/topic` |
| `app-inbound.delete` | 刪除入站資源（先刪 RDP，再刪佇列） |

**必填輸入：**

| 變數 | 說明 | 範例 |
|---|---|---|
| `domain` | 業務領域 | `CENTRAL` |
| `system` | 系統名稱 | `APP` |
| `system_topic` | 主題識別碼 | `SITEB.ORDERS.ORDER-DETAIL` |

**建立的資源：** 除了佇列組合和存取控制資源（Client Profile、per-user ACL Profile、Client Username、發佈例外）外，入站範本還會建立 REST Delivery Point（RDP）、REST Consumer（HTTP 端點）和 Queue Binding（連接佇列與 RDP）。這建立了從 Solace 到後端 REST API 的完整訊息遞送管道。

> 完整參數與動作參考請見 **[docs/template-app-inbound.md](template-app-inbound.md)**。

---

## 9. 可用模組

共有 **225 個動作、86 種物件家族**，每種家族對應 SEMP v2 設定 API 所提供的一種物件型別。完整參考文件（含每個動作的參數表）由 Broker 結構描述產生，並納入版本控制，位於 **[docs/modules/index.md](modules/index.md)**。執行 `semp-workflow list-modules` 可列出您手上這份執行檔所支援的模組。

動作的命名格式為 `<家族>.<動詞>`：

| 動詞 | 何時可用 | 行為 |
|---|---|---|
| `add` | 物件可被建立時 | 建立物件；若已存在則跳過。 |
| `update` | 物件建立後仍可變更時 | 比對目前狀態，只送出有差異的部分；若已符合則跳過，物件不存在則失敗。 |
| `delete` | 物件可被移除時 | 刪除物件；若不存在則跳過。 |

大多數家族支援全部三種動詞。部分家族僅支援建立與移除 —— 訂閱、ACL 例外、受信任的通用名稱（Trusted Common Name）—— 因為 SEMP 在這些物件建立後並無可變更的內容。Broker 設定物件僅支援 `update`，因為它無法被建立或刪除。

### 內建範本所使用的模組

以下是內建 `app-inbound` 與 `app-outbound` 範本所使用的模組，也是最快上手的一組。其餘模組請參閱產生的參考文件。

| 模組 | 說明 |
|---|---|
| `msg_vpn_queue.add` | 建立佇列；若已存在則跳過。 |
| `msg_vpn_queue.delete` | 刪除佇列；若不存在則跳過。 |
| `msg_vpn_queue_subscription.add` | 建立訂閱；若已存在則跳過。 |
| `msg_vpn_rest_delivery_point.add` | 建立 RDP；若已存在則跳過。 |
| `msg_vpn_rest_delivery_point.delete` | 刪除 RDP；若不存在則跳過。 |
| `msg_vpn_rest_delivery_point_rest_consumer.add` | 建立 REST Consumer；若已存在則跳過。 |
| `msg_vpn_rest_delivery_point_queue_binding.add` | 建立佇列繫結（Queue Binding）；若已存在則跳過。 |
| `msg_vpn_acl_profile.add` | 建立 ACL Profile；若已存在則跳過。 |
| `msg_vpn_acl_profile.delete` | 刪除 ACL Profile；若不存在則跳過。 |
| `msg_vpn_acl_profile_publish_topic_exception.add` | 建立發佈主題例外；若已存在則跳過。 |
| `msg_vpn_acl_profile_publish_topic_exception.delete` | 刪除發佈主題例外；若不存在則跳過。 |
| `msg_vpn_client_profile.add` | 建立 Client Profile；若已存在則跳過。 |
| `msg_vpn_client_profile.update` | 更新現有 Client Profile；物件不存在則失敗，已符合預期則跳過。 |
| `msg_vpn_client_username.add` | 建立 Client Username；若已存在則跳過。 |
| `msg_vpn_client_username.delete` | 刪除 Client Username；若不存在則跳過。 |
| `msg_vpn_client_username.update` | 更新現有 Client Username；物件不存在則失敗，已符合預期則跳過。 |

### 命名規則

動作與屬性名稱皆與 [Solace Terraform provider](https://registry.terraform.io/providers/SolaceProducts/solacebroker/latest/docs) 一致，因此同一套拼寫方式可在兩種工具中通用。

家族名稱是 SEMP 物件名稱的 snake_case 形式：`MsgVpnQueue` 對應 `msg_vpn_queue`、`MsgVpnTopicEndpoint` 對應 `msg_vpn_topic_endpoint`、`DmrClusterLink` 對應 `dmr_cluster_link`。這正是該 provider 的資源名稱去掉 `solacebroker_` 前綴後的結果，因此 `solacebroker_msg_vpn_queue` 與 `msg_vpn_queue.add` 指的是同一個物件。

屬性遵循相同規則：SEMP 屬性 `egressEnabled` 寫成 `egress_enabled`，`maxMsgSpoolUsage` 寫成 `max_msg_spool_usage`。[docs/modules/index.md](modules/index.md) 的模組參考列出每個動作每個屬性的確切拼寫。

#### 從舊版升級

依舊版撰寫的工作流程無需修改即可繼續使用，升級時不需要重寫任何內容：

- 原有的十個簡短家族名稱仍可解析：`queue`、`q_sub`、`rdp`、`rdp_rc`、`rdp_qb`、`client_profile`、`client_username`、`acl_profile`、`acl_publish_exception`、`acl_subscribe_exception`。
- camelCase 屬性名稱仍被接受，因此 `egressEnabled` 與 `egress_enabled` 等效。

兩種拼寫都可使用，但只有標準名稱會被列出：`list-modules` 與模組參考僅顯示標準名稱，新範本也應採用標準名稱。請勿在同一個 `args` 區塊中同時提供同一屬性的兩種拼寫 —— 這會被直接拒絕而非自動取捨，因為無從判斷你指的是哪一個值。

### 指定訊息 VPN

VPN 範圍的動作會使用設定檔中的 `msg_vpn`。若要在單一動作中指定不同的 VPN，請明確傳入 `msg_vpn_name`。當工作流程先以 `msg_vpn.add` 建立 VPN 再在其中建立物件時，這一點尤其重要。

**執行結果狀態：**

| 狀態 | 說明 |
|---|---|
| `changed` | Broker 已被變更 |
| `skipped` | 已是目標狀態，未送出任何請求 |
| `dryrun` | 試運行模式：顯示將執行的操作 |
| `failed` | 在本機被驗證擋下，或被 Broker 拒絕 |

由於 `update` 會先比對再送出，同一份設定連續套用兩次時，第一次會回報 `changed`，第二次則回報 `skipped`。若第二次仍回報 `changed`，表示確實有內容每次都不同 —— 請參閱「故障排除」。

---

## 10. 常見情境範例

以下範例均使用內建的 APP 範本（`app-outbound.yaml` 和 `app-inbound.yaml`）。

### 情境一：建立單一 APP 出站佇列組合

**使用情境：** 為特定業務主題建立新的出站訊息流程（Solace -> APP）。

```yaml
# config.yaml
semp:
  host: "https://broker.example.com:943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false

global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_client_profile: "cp-it-user"
  default_acl_profile: "acl-it-user-{{ inputs.aem_client_username }}"

templates_dir: "templates"

workflows:
  - template: "app-outbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"
```

**逐步執行結果：**
1. 建立 Client Profile `cp-it-user`（已存在則跳過）
2. 更新 `cp-it-user`，啟用 guaranteed send/receive
3. 建立 ACL Profile `acl-it-user-svc-app-client`，設定允許連線、禁止發佈、禁止訂閱（已存在則跳過）
4. 建立 Client Username `svc-app-client`，關聯至兩個 Profile（已存在則跳過）
5. 新增發佈主題例外 `SITEA/APP/AIF/SITEB.ORDERS.ORDER-CREATE` 至 ACL Profile（已存在則跳過）
6. 建立服務佇列 `TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
7. 建立鏡像佇列 `MIRROR/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
8. 建立死信佇列 `DMQ/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
9. 訂閱服務佇列至 `SITEA/APP/AIF/SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
10. 訂閱鏡像佇列至相同主題（已存在則跳過）

重新執行時所有步驟均為 `SKIPPED`——不會有任何變更。

```bash
# 試運行確認
semp-workflow run -c config.yaml --dry-run

# 確認無誤後正式執行
semp-workflow run -c config.yaml
```

---

### 情境二：建立 APP 入站流程（佇列 + RDP + REST 遞送）

**使用情境：** 建立入站訊息流程（APP -> Solace -> 後端 REST 服務），將訊息遞送至 HTTP 端點。

```yaml
workflows:
  - template: "app-inbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      rc_remote_host: "app-backend.internal"
      rc_remote_port: 443
      rc_tls_enabled: true
```

**逐步執行結果：**
1. 建立/更新 Client Profile `cp-it-user`，啟用 guaranteed send/receive
2. 建立 ACL Profile `acl-it-user-APP-AIF-CLIENT`（已存在則跳過）
3. 建立 Client Username `APP-AIF-CLIENT`（已存在則跳過）
4. 新增訂閱主題的發佈主題例外（已存在則跳過）
5. 建立服務佇列 `FROM-CENTRAL-APP-SITEB.ORDERS.ORDER-CREATE`，擁有者為 RDP（已存在則跳過）
6. 建立鏡像佇列和死信佇列（已存在則跳過）
7. 兩個佇列訂閱 `CENTRAL/APP/SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
8. 建立 REST Delivery Point `RDP/FROM-CENTRAL-APP-SITEB.ORDERS.ORDER-CREATE`（已存在則跳過）
9. 建立 REST Consumer，指向 `app-backend.internal:443`，啟用 TLS（已存在則跳過）
10. 繫結服務佇列至 RDP（已存在則跳過）

完整管道：訊息到達主題 -> 服務佇列（經由訂閱）-> RDP 透過佇列繫結取得訊息 -> REST Consumer 以 HTTP POST 遞送至後端。

---

### 情境三：批次建立多個工作流程

**使用情境：** 一次建立多個訊息流程，例如在初始環境佈建時。

```yaml
workflows:
  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "ORDER.CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "ORDER.UPDATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  - template: "app-inbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "ORDER.CONFIRM"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      rc_remote_host: "app-backend.internal"
```

**執行結果：** 三個工作流程依序執行。前兩個建立出站佇列組合（共用 `cp-it-user` Profile，各自的 ACL Profile）；第三個建立入站流程，包含 RDP 和 REST Consumer。前面工作流程已建立的存取控制資源會自動跳過。

```bash
semp-workflow run -c config.yaml
```

---

### 情境四：使用全域變數統一管理設定

**使用情境：** 多個工作流程共用相同的後端連線、命名慣例和存取控制預設值。在 `global_vars` 中定義一次即可。

```yaml
global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_client_profile: "cp-it-user"
  default_acl_profile: "acl-it-user-{{ inputs.aem_client_username }}"
  default_rc_remote_host: "app-backend.internal"
  default_rc_remote_port: 443
  default_rc_tls_enabled: true
  default_queue_owner: "svc-app-client"

workflows:
  - template: "app-inbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-DETAIL"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      # 選填覆蓋項引用 global_vars：
      #client_profile_name: "{{ global_vars.default_client_profile }}"
      #acl_profile_name: "{{ global_vars.default_acl_profile }}"
      rc_remote_host: "{{ global_vars.default_rc_remote_host }}"
      rc_remote_port: "{{ global_vars.default_rc_remote_port }}"
      rc_tls_enabled: "{{ global_vars.default_rc_tls_enabled }}"
```

**運作方式：** `{{ global_vars.X }}` 運算式在執行時解析。如 `default_acl_profile` 包含 `{{ inputs.aem_client_username }}`，會經過兩輪 Jinja2 渲染——第一輪展開全域變數，第二輪替換輸入值。只需在一處修改設定，所有工作流程即可套用。

---

### 情境五：刪除資源

**使用情境：** 停用訊息流程並清理所有 Broker 資源。

```yaml
workflows:
  # 刪除出站流程
  - template: "app-outbound.delete"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      aem_client_username: "svc-app-client"

  # 刪除入站流程
  - template: "app-inbound.delete"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-DETAIL"
      aem_client_username: "APP-AIF-CLIENT"
```

**執行結果：**

出站刪除：移除服務佇列、鏡像佇列、死信佇列，然後移除發佈主題例外、Client Username 和 ACL Profile。

入站刪除：先移除 RDP（連帶移除 Consumer 和 Binding），再移除三個佇列，最後移除存取控制資源。

不存在的資源會被跳過。共用的 Client Profile `cp-it-user` **不會被刪除**，因為其他工作流程可能仍在使用。

> **提示**：刪除前請務必先試運行：
> ```bash
> semp-workflow run -c config.yaml --dry-run
> ```

---

### 情境六：循序遞送 vs 並發遞送

**使用情境：** 某個主題需要循序遞送（訊息依序處理，不重試），另一個主題需要並發遞送（訊息可重試最多 5 次）。

```yaml
workflows:
  # 循序：max_redelivery = 0（永遠保留，不轉移至 DMQ）
  - template: "app-outbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "CRITICAL.ORDER.CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  # 並發：max_redelivery = 5（重試 5 次後轉移至 DMQ）
  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "BATCH.STATUS.UPDATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"
```

**差異：** 兩個範本建立完全相同的資源（佇列、訂閱、存取控制）。唯一的差異是 `service_queue_max_redelivery`：`0` 表示循序（訊息永遠保留在佇列中直到被消費），`5` 表示並發（失敗的訊息在重試 5 次後轉移至死信佇列）。

---

## 11. 故障排除

### 問題：找不到範本（Template not found）

```
TemplateError: Template 'app-outbound.new-seq' not found.
```

**原因與解決方式：**
- 確認 `config.yaml` 中的 `templates_dir` 路徑正確（相對於 config.yaml 所在目錄）
- 確認範本目錄中確實存在 `app-outbound.yaml`
- 未設定 `templates_dir` / `--templates-dir` 時，工具會使用內嵌範本；執行 `semp-workflow init` 可將內建範本匯出以供自訂

---

### 問題：未提供必填輸入（Required input not provided）

```
TemplateError: Required input 'domain' not provided
```

**解決方式：** 在 `config.yaml` 工作流程的 `inputs:` 區塊中補上缺少的必填變數。

---

### 問題：未預期的輸入變數（Unexpected inputs）

```
TemplateError: Unexpected inputs: my_typo_var
```

**解決方式：** 確認輸入變數名稱拼寫正確，需與範本 `optional:` 區塊中定義的名稱完全相符。

---

### 問題：連線失敗（Connection error）

```
SEMPError: Connection refused / SSL error
```

**解決方式：**
- 確認 `semp.host` URL 格式正確（含通訊協定 `https://` 及正確埠號）
- 自簽憑證環境請設定 `verify_ssl: false`
- 確認帳號密碼正確，且有 Message VPN 的管理權限

---

### 問題：未知的參數（Unknown argument）

```
unknown argument: acessType (see docs/modules/msg_vpn_queue.md for the accepted attributes)
```

**原因：** 該動作傳入了不屬於此物件的參數。參數會依 Broker 結構描述中該物件的完整屬性集合檢查，因此幾乎都是拼字錯誤。

**解決方式：** 開啟 [docs/modules/](modules/index.md) 中該家族的頁面，對照參數表確認拼字。請注意 SEMP 屬性名稱採用小駝峰式（camelCase），而唯讀屬性不接受作為輸入 —— 因為 Broker 同樣不接受。

---

### 問題：屬性無法在 update 中設定

```
'flavour' can only be set when the object is created, so it cannot appear in an update
```

**原因：** SEMP 將部分屬性標記為建立後即固定。舊版會靜默地將其忽略，因而對一項並未實際發生的變更回報成功。

**解決方式：** 從 `update` 動作中移除該屬性。若該值確實必須變更，則需刪除並重新建立該物件 —— 請將這兩個步驟明確寫入工作流程，使其一目了然。

---

### 問題：值超過長度上限

```
'queue_name' value is 203 bytes but the broker limit is 200: '...'
```

**原因：** Broker 對名稱與主題字串設有長度上限。此檢查在本機執行，因此請求不會被送出。

**解決方式：** 縮短該值。上限以**位元組（byte）**計算而非字元數，因此含有重音符號或非拉丁字元的名稱，會比其顯示長度更早達到上限。

---

### 問題：第二次執行仍回報 `changed`

`update` 會比對目前狀態，若物件已符合指定值則應回報 `skipped`，因此同一份設定套用兩次時，第二次應回報未變更。

**可能原因：**
- **Broker 不會回傳的屬性。** 密碼及其他唯寫（write-only）或不透明（opaque）屬性無法讀回比對，因此每次都會重新送出，動作也隨之回報 `changed`。這屬於預期行為。
- **Broker 會正規化的值。** 若 Broker 儲存的形式與您提供的不同，每次比對都會出現差異。請對 Broker 執行 `GET` 查看該屬性，並改用其儲存的形式。

**判斷方式：** 以 `--dry-run` 執行，試運行訊息會列出它將送出的屬性。

---

### 問題：變數未解析（Unresolved Jinja2 expression）

```
WorkflowError: Input 'queue_name' still contains an unresolved Jinja2 expression
```

**解決方式：**
- 確認引用的輸入變數已存在（無誤字）
- 避免循環引用（如 A 的預設值引用 B，B 的預設值又引用 A）

---

### 試運行模式

在執行任何變更前，強烈建議先使用試運行模式確認操作內容：

```bash
semp-workflow run -c config.yaml --dry-run --verbose
```
