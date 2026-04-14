# TinybirdService handles sending heartbeat events to Tinybird's Events API
# for real-time analytics powered by ClickHouse.
#
# Configure via environment variables:
#   TINYBIRD_API_URL   - Tinybird API base URL (default: https://api.tinybird.co)
#   TINYBIRD_TOKEN     - Workspace admin token (used as fallback)
#
# Per-user tokens are stored in users.tinybird_token for multi-tenant setups.
class TinybirdService
  TINYBIRD_API_URL = ENV.fetch("TINYBIRD_API_URL", "https://api.tinybird.co")
  DATASOURCE_NAME = "heartbeat_events"

  # Enqueue async ingestion via ActiveJob
  def self.ingest_async(heartbeat)
    return unless tinybird_configured?(heartbeat.user)
    TinybirdIngestJob.perform_later(heartbeat.id)
  end

  # Synchronously ingest a heartbeat to Tinybird
  def self.ingest(heartbeat)
    token = token_for(heartbeat.user)
    return false unless token

    payload = build_payload(heartbeat)
    response = post_to_tinybird(token, payload)
    response.success?
  rescue Faraday::Error => e
    Rails.logger.error("TinybirdService error: #{e.message}")
    false
  end

  # Bulk ingest multiple heartbeats
  def self.bulk_ingest(heartbeats)
    return false if heartbeats.empty?

    user = heartbeats.first.user
    token = token_for(user)
    return false unless token

    ndjson = heartbeats.map { |hb| build_payload(hb).to_json }.join("\n")
    response = connection(token).post("/v0/events?name=#{DATASOURCE_NAME}") do |req|
      req.body = ndjson
    end
    response.success?
  rescue Faraday::Error => e
    Rails.logger.error("TinybirdService bulk error: #{e.message}")
    false
  end

  private

  def self.tinybird_configured?(user)
    token_for(user).present?
  end

  def self.token_for(user)
    user.tinybird_token.presence || ENV["TINYBIRD_TOKEN"].presence
  end

  def self.build_payload(heartbeat)
    {
      id: heartbeat.id,
      user_id: heartbeat.user_id,
      project_id: heartbeat.project_id,
      project_name: heartbeat.project&.name,
      agent_type: heartbeat.agent_type,
      entity: heartbeat.entity,
      entity_type: heartbeat.entity_type,
      language: heartbeat.language,
      branch: heartbeat.branch,
      operating_system: heartbeat.operating_system,
      machine: heartbeat.machine,
      time: heartbeat.time,
      time_iso: heartbeat.timestamp.iso8601,
      lines_added: heartbeat.lines_added,
      lines_deleted: heartbeat.lines_deleted,
      tokens_used: heartbeat.tokens_used,
      cost_usd: heartbeat.cost_usd&.to_f,
      is_write: heartbeat.is_write,
      metadata: heartbeat.metadata.to_json
    }
  end

  def self.post_to_tinybird(token, payload)
    connection(token).post("/v0/events?name=#{DATASOURCE_NAME}") do |req|
      req.body = payload.to_json
    end
  end

  def self.connection(token)
    Faraday.new(url: TINYBIRD_API_URL) do |f|
      f.request :json
      f.response :raise_error
      f.headers["Authorization"] = "Bearer #{token}"
      f.headers["Content-Type"] = "application/json"
    end
  end
end
