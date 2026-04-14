require "digest"

class DeviceAuthorization < ApplicationRecord
  STATUSES = %w[pending approved claimed expired].freeze
  DEFAULT_INTERVAL = 5
  DEFAULT_EXPIRES_IN = 15.minutes
  USER_CODE_ALPHABET = %w[A B C D E F G H J K M N P Q R S T U V W X Y 2 3 4 5 6 7 8 9].freeze

  belongs_to :user, optional: true
  belongs_to :api_key, optional: true

  attr_reader :device_code

  before_validation :assign_defaults, on: :create

  validates :device_code_digest, presence: true, uniqueness: true
  validates :user_code, presence: true, uniqueness: true
  validates :status, presence: true, inclusion: { in: STATUSES }
  validates :expires_at, presence: true
  validates :interval, presence: true, numericality: { greater_than: 0 }

  scope :pending, -> { where(status: "pending") }

  def self.create_pending!(machine_name: nil, expires_in: DEFAULT_EXPIRES_IN, interval: DEFAULT_INTERVAL)
    create!(machine_name: machine_name, expires_at: Time.current + expires_in, interval: interval)
  end

  def self.find_by_device_code(device_code)
    find_by(device_code_digest: digest_device_code(device_code))
  end

  def self.digest_device_code(device_code)
    Digest::SHA256.hexdigest(device_code.to_s)
  end

  def self.generate_device_code
    SecureRandom.urlsafe_base64(32)
  end

  def self.generate_user_code
    token = Array.new(8) { USER_CODE_ALPHABET.sample }.join
    "#{token[0, 4]}-#{token[4, 4]}"
  end

  def pending?
    status == "pending"
  end

  def approved?
    status == "approved"
  end

  def claimed?
    status == "claimed"
  end

  def expired?
    status == "expired" || (expires_at <= Time.current && !claimed?)
  end

  def expire_if_needed!
    return false unless expires_at <= Time.current && %w[pending approved].include?(status)

    update!(status: "expired")
    true
  end

  def approve!(user:, api_key:)
    raise ActiveRecord::RecordInvalid, self if expired?
    raise ActiveRecord::RecordInvalid, self unless pending?

    update!(
      status: "approved",
      user: user,
      api_key: api_key,
      approved_at: Time.current
    )
  end

  def claim!(api_base_url:)
    raise ActiveRecord::RecordInvalid, self if expired?
    raise ActiveRecord::RecordInvalid, self unless approved?
    raise ActiveRecord::RecordInvalid, self unless api_key&.active?

    payload = {
      api_key: api_key.key,
      api_base_url: api_base_url,
      user: {
        id: user.id,
        email: user.email,
        name: user.display_name,
        display_name: user.display_name,
        full_name: user.name,
        timezone: user.timezone,
        created_at: user.created_at
      }
    }

    update!(status: "claimed", claimed_at: Time.current)
    payload
  end

  private

  def assign_defaults
    self.status ||= "pending"
    self.interval ||= DEFAULT_INTERVAL
    self.expires_at ||= Time.current + DEFAULT_EXPIRES_IN
    self.user_code ||= generate_unique_user_code
    assign_device_code_digest
  end

  def assign_device_code_digest
    return if device_code_digest.present?

    @device_code = self.class.generate_device_code
    self.device_code_digest = self.class.digest_device_code(@device_code)
  end

  def generate_unique_user_code
    loop do
      candidate = self.class.generate_user_code
      break candidate unless self.class.exists?(user_code: candidate)
    end
  end
end
