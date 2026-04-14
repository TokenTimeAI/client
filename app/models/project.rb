class Project < ApplicationRecord
  belongs_to :user
  has_many :heartbeat_events, dependent: :nullify

  COLORS = %w[#6366f1 #8b5cf6 #ec4899 #ef4444 #f97316 #eab308 #22c55e #14b8a6 #3b82f6 #06b6d4].freeze

  validates :name, presence: true
  validates :name, uniqueness: { scope: :user_id }

  before_create :assign_color

  private

  def assign_color
    self.color ||= COLORS.sample
  end
end
