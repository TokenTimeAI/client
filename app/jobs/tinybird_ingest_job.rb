class TinybirdIngestJob < ApplicationJob
  queue_as :default

  retry_on Faraday::Error, wait: :polynomially_longer, attempts: 3

  def perform(heartbeat_id)
    heartbeat = HeartbeatEvent.find_by(id: heartbeat_id)
    return unless heartbeat

    TinybirdService.ingest(heartbeat)
  end
end
