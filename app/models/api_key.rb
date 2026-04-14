class ApiKey < ApplicationRecord
  belongs_to :user

  before_create :generate_key

  scope :active, -> { where(active: true) }

  validates :name, presence: true
  validates :key, uniqueness: true

  def self.authenticate(token)
    key = find_by(key: token, active: true)
    return nil unless key

    key.touch(:last_used_at)
    key.user
  end

  private

  def generate_key
    self.key ||= "tt_#{SecureRandom.hex(32)}"
  end
end
