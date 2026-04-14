class HeartbeatEvent < ApplicationRecord
  belongs_to :user
  belongs_to :project, optional: true

  AGENT_TYPES = %w[claude_code codex cursor copilot codeium continue aider devin custom].freeze
  ENTITY_TYPES = %w[file app domain url].freeze

  validates :agent_type, presence: true
  validates :time, presence: true
  validates :entity, presence: true

  scope :for_period, ->(start_time, end_time) {
    where("time >= ? AND time <= ?", start_time.to_f, end_time.to_f)
  }

  scope :by_agent, ->(agent) { where(agent_type: agent) }
  scope :by_language, ->(lang) { where(language: lang) }

  def timestamp
    Time.at(time).utc
  end
end
