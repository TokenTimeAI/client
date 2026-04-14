class User < ApplicationRecord
  ACTIVE_SUBSCRIPTION_STATUSES = %w[active trialing past_due].freeze

  OAUTH_PROVIDERS = %i[github google_oauth2].freeze

  # Include default devise modules. Others available are:
  # :confirmable, :lockable, :timeoutable, :trackable and :omniauthable
  devise :database_authenticatable, :registerable,
         :recoverable, :rememberable, :validatable,
         :omniauthable, omniauth_providers: OAUTH_PROVIDERS

  has_many :api_keys, dependent: :destroy
  has_many :projects, dependent: :destroy
  has_many :heartbeat_events, dependent: :destroy

  validates :name, presence: true
  validates :uid, uniqueness: { scope: :provider }, allow_nil: true

  def self.from_omniauth(auth)
    auth = auth.to_h.deep_symbolize_keys
    provider = auth[:provider].to_s
    uid = auth[:uid].to_s

    return oauth_error_user("OAuth response is missing provider details") if provider.blank? || uid.blank?

    if (user = find_by(provider: provider, uid: uid))
      return user
    end

    email = verified_email_from_oauth(auth)
    if email.blank?
      return oauth_error_user("#{provider_name(provider)} did not return a verified email address. Sign in with email/password or use a #{provider_name(provider)} account with a verified email.")
    end

    if (user = find_by(email: email))
      return oauth_error_user("#{provider_name(provider)} sign-in could not be linked to this account. Sign in with email/password first.") unless linkable_oauth_account?(user, provider: provider, uid: uid)

      user.provider = provider
      user.uid = uid
      user.save
      return user
    end

    user = new(oauth_attributes_for(new, auth, email).merge(provider: provider, uid: uid))
    user.save
    user
  end

  def self.verified_email_from_oauth(auth)
    auth = auth.to_h.deep_symbolize_keys

    case auth[:provider].to_s
    when "google_oauth2"
      email = auth.dig(:info, :email)
      auth.dig(:info, :email_verified) ? email&.downcase : nil
    when "github"
      preferred_email = auth.dig(:info, :email).to_s.downcase
      verified_emails = Array(auth.dig(:extra, :all_emails)).filter_map do |entry|
        entry = entry.to_h.symbolize_keys
        entry[:email].to_s.downcase if entry[:verified]
      end

      verified_emails.find { |email| email == preferred_email }.presence || verified_emails.first
    else
      nil
    end
  end

  def self.oauth_attributes_for(user, auth, email)
    attributes = {
      email: email.downcase,
      name: user.name.presence || oauth_name(auth, email)
    }

    attributes[:password] = Devise.friendly_token.first(32) if user.new_record?
    attributes
  end

  def self.oauth_name(auth, email)
    auth.dig(:info, :name).presence || auth.dig(:info, :nickname).presence || email.to_s.split("@").first
  end

  def self.provider_name(provider)
    { "github" => "GitHub", "google_oauth2" => "Google" }.fetch(provider.to_s, provider.to_s.titleize)
  end

  def self.oauth_error_user(message)
    new.tap { |user| user.errors.add(:base, message) }
  end

  def self.linkable_oauth_account?(user, provider:, uid:)
    return true if user.provider.blank? && user.uid.blank?

    user.provider == provider && (user.uid.blank? || user.uid == uid)
  end

  def display_name
    name.presence || email.split("@").first
  end

  def active_api_key
    api_keys.active.first
  end

  def subscription_active?
    ACTIVE_SUBSCRIPTION_STATUSES.include?(stripe_subscription_status.to_s)
  end
  alias_method :subscribed?, :subscription_active?

  def sync_stripe_subscription!(subscription)
    cancel_at_period_end = stripe_attribute(subscription, :cancel_at_period_end)

    update!(
      stripe_customer_id: stripe_attribute(subscription, :customer) || stripe_customer_id,
      stripe_subscription_id: stripe_attribute(subscription, :id) || stripe_subscription_id,
      stripe_subscription_status: stripe_attribute(subscription, :status) || stripe_subscription_status,
      stripe_price_id: subscription_price_id(subscription) || stripe_price_id,
      subscription_current_period_end: timestamp_to_time(stripe_attribute(subscription, :current_period_end)) || subscription_current_period_end,
      subscription_cancel_at_period_end: cancel_at_period_end.nil? ? subscription_cancel_at_period_end : ActiveModel::Type::Boolean.new.cast(cancel_at_period_end)
    )
  end

  def mark_subscription_canceled!(subscription = nil)
    update!(
      stripe_customer_id: stripe_attribute(subscription, :customer) || stripe_customer_id,
      stripe_subscription_id: stripe_attribute(subscription, :id) || stripe_subscription_id,
      stripe_subscription_status: stripe_attribute(subscription, :status) || "canceled",
      stripe_price_id: subscription_price_id(subscription) || stripe_price_id,
      subscription_current_period_end: timestamp_to_time(stripe_attribute(subscription, :current_period_end)) || subscription_current_period_end,
      subscription_cancel_at_period_end: false
    )
  end

  private

  def stripe_attribute(record, key)
    return if record.blank?

    if record.respond_to?(key)
      record.public_send(key)
    elsif record.respond_to?(:[])
      record[key.to_s] || record[key.to_sym]
    end
  end

  def subscription_price_id(subscription)
    items = stripe_attribute(subscription, :items)
    item = stripe_attribute(items, :data)&.first
    price = stripe_attribute(item, :price)

    stripe_attribute(price, :id)
  end

  def timestamp_to_time(timestamp)
    return if timestamp.blank?

    Time.zone.at(timestamp)
  end
end
