Feature: 訂單狀態變更微服務（Go + RabbitMQ）
  為了正確管理訂單狀態生命週期
  作為訂單狀態微服務
  當收到外部事件或定時器觸發時
  我希望能依據狀態機規則安全地更新訂單狀態，並透過 MQ 同步到主後端資料庫

  # 狀態說明：
  # Created            使用者建立訂單
  # WaitingForShipment 已付款，等待出貨（支付服務完成後調用此服務）
  # InTransit          寄件中（物流運輸中）
  # WaitPickup         已送達門市，等待取貨
  # Completed          已取貨（訂單完成）

  # 訂單狀態轉換規則：
  # Created           → WaitingForShipment（支付完成後）
  # WaitingForShipment→ InTransit
  # InTransit         → WaitPickup
  # WaitPickup        → Completed
  #
  # 注意：此服務不會修改訂單狀態為 WaitingForPayment
  # 支付服務完成後才會調用此服務，將訂單從 Created 轉換為 WaitingForShipment
  #
  # 任何不在上述清單中的轉換都視為不合法（例如 Completed → WaitPickup）

  Background:
    Given 系統中存在一筆訂單 "ORDER-1001"
    And 該訂單目前狀態為 "Created"
    And 系統已啟用訂單狀態微服務，並連線到 RabbitMQ
    And 系統會在每次狀態變更時自動新增一筆 OrderStep 記錄
    And 系統會在每次狀態變更時發布一個訂單狀態變更事件到 RabbitMQ
    And 主後端服務會接收 RabbitMQ 中的訂單狀態變更事件並同步更新資料庫

  ############################################################
  # Scenario 1: 支付完成事件處理
  ############################################################

  Scenario: 處理支付完成事件，將訂單從 Created 轉換為 WaitingForShipment
    Given 訂單 "ORDER-1001" 狀態為 "Created"
    When 微服務收到一個 "PaymentCompleted" 事件，內容如下:
      """
      {
        "orderId": "ORDER-1001"
      }
      """
    Then 訂單 "ORDER-1001" 的狀態應該更新為 "WaitingForShipment"
    And 應新增一筆 OrderStep 記錄，fromStatus 為 "Created"，toStatus 為 "WaitingForShipment"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "Created"，toStatus 為 "WaitingForShipment"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

  ############################################################
  # Scenario 2: 定時器自動更新訂單狀態（隨機方式）
  ############################################################

  Scenario: 定時器每 10 秒檢查並使用隨機方式更新訂單狀態
    Given 訂單 "ORDER-2001" 狀態為 "WaitingForShipment"
    And 系統中有 3 筆 "WaitingForShipment" 狀態的訂單
    When 定時器開始運行，每 "10" 秒檢查一次資料庫中的訂單狀態
    And 定時器使用隨機方式決定每筆訂單是否更新（50% 機率）
    Then 在下一次定時器執行時，部分訂單的狀態可能會更新為 "InTransit"
    And 如果訂單狀態更新，應新增一筆 OrderStep 記錄，fromStatus 為 "WaitingForShipment"，toStatus 為 "InTransit"
    And 如果訂單狀態更新，應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "WaitingForShipment"，toStatus 為 "InTransit"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

  Scenario: 定時器處理 InTransit 到 WaitPickup 的狀態轉換
    Given 訂單 "ORDER-2002" 狀態為 "InTransit"
    And 系統中有 2 筆 "InTransit" 狀態的訂單
    When 定時器開始運行，每 "10" 秒檢查一次資料庫中的訂單狀態
    And 定時器使用隨機方式決定每筆訂單是否更新（50% 機率）
    Then 在下一次定時器執行時，部分訂單的狀態可能會更新為 "WaitPickup"
    And 如果訂單狀態更新，應新增一筆 OrderStep 記錄，fromStatus 為 "InTransit"，toStatus 為 "WaitPickup"
    And 如果訂單狀態更新，應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "InTransit"，toStatus 為 "WaitPickup"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

  Scenario: 定時器處理 WaitPickup 到 Completed 的狀態轉換
    Given 訂單 "ORDER-2003" 狀態為 "WaitPickup"
    And 系統中有 1 筆 "WaitPickup" 狀態的訂單
    When 定時器開始運行，每 "10" 秒檢查一次資料庫中的訂單狀態
    And 定時器使用隨機方式決定每筆訂單是否更新（50% 機率）
    Then 在下一次定時器執行時，部分訂單的狀態可能會更新為 "Completed"
    And 如果訂單狀態更新，應新增一筆 OrderStep 記錄，fromStatus 為 "WaitPickup"，toStatus 為 "Completed"
    And 如果訂單狀態更新，應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "WaitPickup"，toStatus 為 "Completed"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

  ############################################################
  # Scenario 3: 完整的訂單生命週期（支付完成 + 定時器自動更新）
  ############################################################

  Scenario: 完整正常流程 - 從支付完成到訂單完成
    Given 訂單 "ORDER-3001" 狀態為 "Created"
    When 微服務收到一個 "PaymentCompleted" 事件，內容如下:
      """
      {
        "orderId": "ORDER-3001"
      }
      """
    Then 訂單 "ORDER-3001" 的狀態應該更新為 "WaitingForShipment"
    And 應新增一筆 OrderStep 記錄，fromStatus 為 "Created"，toStatus 為 "WaitingForShipment"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "Created"，toStatus 為 "WaitingForShipment"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

    When 定時器執行，檢查 "WaitingForShipment" 狀態的訂單
    And 隨機決定更新訂單 "ORDER-3001" 的狀態
    Then 訂單 "ORDER-3001" 的狀態應該更新為 "InTransit"
    And 應新增一筆 OrderStep 記錄，fromStatus 為 "WaitingForShipment"，toStatus 為 "InTransit"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "WaitingForShipment"，toStatus 為 "InTransit"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

    When 定時器執行，檢查 "InTransit" 狀態的訂單
    And 隨機決定更新訂單 "ORDER-3001" 的狀態
    Then 訂單 "ORDER-3001" 的狀態應該更新為 "WaitPickup"
    And 應新增一筆 OrderStep 記錄，fromStatus 為 "InTransit"，toStatus 為 "WaitPickup"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "InTransit"，toStatus 為 "WaitPickup"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

    When 定時器執行，檢查 "WaitPickup" 狀態的訂單
    And 隨機決定更新訂單 "ORDER-3001" 的狀態
    Then 訂單 "ORDER-3001" 的狀態應該更新為 "Completed"
    And 應新增一筆 OrderStep 記錄，fromStatus 為 "WaitPickup"，toStatus 為 "Completed"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，其內容 fromStatus 為 "WaitPickup"，toStatus 為 "Completed"
    And 主後端服務應該接收到該事件並同步更新訂單狀態

    And 最終訂單 "ORDER-3001" 的 OrderStep 記錄應該依順序包含以下轉換:
      | fromStatus         | toStatus           |
      | Created            | WaitingForShipment |
      | WaitingForShipment | InTransit          |
      | InTransit          | WaitPickup         |
      | WaitPickup         | Completed          |

  ############################################################
  # Scenario 4: 不合法的狀態轉換應被拒絕
  ############################################################

  Scenario: 已完成的訂單不允許回到其他狀態
    Given 訂單 "ORDER-4001" 狀態為 "Completed"
    When 定時器嘗試更新訂單 "ORDER-4001" 的狀態為 "WaitPickup"
    Then 微服務應該拒絕這次狀態變更
    And 應記錄一筆錯誤日誌，內容包含 "invalid status transition"
    And 訂單 "ORDER-4001" 的狀態應該仍然為 "Completed"
    And 不應新增任何新的 OrderStep 記錄
    And 不應發布任何新的 "OrderStatusChanged" 事件到 RabbitMQ

  Scenario: 從 WaitingForShipment 直接跳成 Completed 應被視為不合法
    Given 訂單 "ORDER-4002" 狀態為 "WaitingForShipment"
    When 定時器嘗試更新訂單 "ORDER-4002" 的狀態為 "Completed"
    Then 微服務應該拒絕這次狀態變更
    And 訂單 "ORDER-4002" 的狀態應該仍然為 "WaitingForShipment"
    And 不應新增任何新的 OrderStep 記錄
    And 不應發布任何新的 "OrderStatusChanged" 事件到 RabbitMQ

  ############################################################
  # Scenario 5: 隨機更新機制驗證
  ############################################################

  Scenario: 驗證隨機更新機制 - 部分訂單可能不會立即更新
    Given 系統中有 5 筆 "WaitingForShipment" 狀態的訂單
    When 定時器執行，使用隨機方式決定每筆訂單是否更新（50% 機率）
    Then 部分訂單的狀態可能會更新為 "InTransit"
    And 部分訂單的狀態可能仍然為 "WaitingForShipment"
    And 在下一次定時器執行時，未更新的訂單仍有機會被更新
    And 每次狀態更新都會發布 "OrderStatusChanged" 事件到 RabbitMQ
    And 主後端服務應該接收到所有更新事件並同步更新對應的訂單狀態

  ############################################################
  # Scenario 6: 狀態機驗證（CanTransitionTo）
  ############################################################

  Scenario Outline: 驗證狀態機允許的轉換
    Given 系統中存在狀態從 "<fromStatus>" 到 "<toStatus>" 的轉換請求，訂單編號為 "ORDER-5001"
    When 微服務檢查狀態機是否允許轉換
    Then 結果應該為 "<allowed>"

    Examples:
      | fromStatus         | toStatus           | allowed |
      | Created            | WaitingForShipment | true    |
      | WaitingForShipment | InTransit          | true    |
      | InTransit          | WaitPickup         | true    |
      | WaitPickup         | Completed          | true    |
      | Created            | WaitingForPayment  | false   |
      | Completed          | WaitPickup         | false   |
      | Completed          | WaitingForShipment | false   |
      | InTransit          | Completed          | false   |
      | WaitingForShipment | Completed          | false   |

  ############################################################
  # Scenario 7: MQ 消息發布與主後端同步驗證
  ############################################################

  Scenario: 驗證狀態更新時會發布消息到 RabbitMQ 並同步到主後端
    Given 訂單 "ORDER-6001" 狀態為 "WaitingForShipment"
    When 定時器執行並決定更新訂單 "ORDER-6001" 的狀態為 "InTransit"
    Then 訂單 "ORDER-6001" 在 Go 服務資料庫中的狀態應該更新為 "InTransit"
    And 應發布一個 "OrderStatusChanged" 事件到 RabbitMQ，包含以下內容:
      """
      {
        "eventType": "OrderStatusChanged",
        "orderId": "ORDER-6001",
        "fromStatus": "WaitingForShipment",
        "toStatus": "InTransit",
        "timestamp": "2025-12-01T12:00:00Z"
      }
      """
    And 主後端服務的 OrderStatusChangedConsumer 應該接收到該消息
    And 主後端服務應該解析消息並調用 SyncOrderStatusFromStateServiceAsync
    And 主後端資料庫中訂單 "ORDER-6001" 的狀態應該同步更新為 "InTransit"
    And 主後端資料庫中應該新增對應的 OrderStep 記錄
