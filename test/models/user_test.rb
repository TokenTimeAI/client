require "test_helper"
require "omniauth/auth_hash"

class UserTest < ActiveSupport::TestCase
  test "is valid with all required fields" do
    user = User.new(email: "user@example.com", password: "password123", name: "Alice")
    assert user.valid?
  end

  test "requires a name" do
    user = User.new(email: "user@example.com", password: "password123")
    assert_not user.valid?
    assert_includes user.errors[:name], "can't be blank"
  end

  test "gets a ULID id on create" do
    user = User.create!(email: "ulid@example.com", password: "password123", name: "ULID Test")
    assert user.id.present?
    assert_equal 26, user.id.length
    assert_match(/\A[0-9A-Z]{26}\z/, user.id)
  end

  test "display_name uses name field" do
    user = User.new(name: "Jane Smith", email: "jane@example.com")
    assert_equal "Jane Smith", user.display_name
  end

  test "display_name falls back to email prefix" do
    user = User.new(name: nil, email: "jane@example.com")
    assert_equal "jane", user.display_name
  end

  test "from_omniauth links a verified oauth email to an existing local account" do
    user = User.create!(email: "alice@example.com", password: "password123", name: "Alice")

    resolved_user = User.from_omniauth(google_auth_hash(email: user.email, uid: "google-123"))

    assert_equal user, resolved_user
    assert_equal "google_oauth2", resolved_user.provider
    assert_equal "google-123", resolved_user.uid
  end

  test "from_omniauth does not overwrite an existing local password" do
    user = User.create!(email: "alice@example.com", password: "password123", name: "Alice")

    User.from_omniauth(google_auth_hash(email: user.email, uid: "google-123"))

    assert user.reload.valid_password?("password123")
  end

  test "from_omniauth creates a new oauth user" do
    assert_difference("User.count", 1) do
      @user = User.from_omniauth(google_auth_hash(email: "new-user@example.com", uid: "google-456", name: "New User"))
    end

    assert @user.persisted?
    assert_equal "new-user@example.com", @user.email
    assert_equal "New User", @user.name
    assert_equal "google_oauth2", @user.provider
    assert_equal "google-456", @user.uid
  end

  test "from_omniauth rejects unverified oauth emails" do
    user = User.from_omniauth(google_auth_hash(email: "unverified@example.com", uid: "google-789", email_verified: false))

    assert_not user.persisted?
    assert_match(/Google did not return a verified email address/, user.errors.full_messages.to_sentence)
  end

  test "subscription_active? is true for active stripe statuses" do
    user = User.new(stripe_subscription_status: "active")
    assert user.subscription_active?

    user.stripe_subscription_status = "trialing"
    assert user.subscription_active?

    user.stripe_subscription_status = "past_due"
    assert user.subscription_active?
  end

  test "subscription_active? is false for inactive statuses" do
    user = User.new(stripe_subscription_status: "canceled")
    assert_not user.subscription_active?

    user.stripe_subscription_status = nil
    assert_not user.subscription_active?
  end

  test "subscribed? aliases subscription_active?" do
    user = User.new(stripe_subscription_status: "trialing")

    assert user.subscribed?
  end

  test "sync_stripe_subscription! stores billing details" do
    user = User.create!(email: "billing@example.com", password: "password123", name: "Billing User")
    subscription = stripe_subscription(
      id: "sub_123",
      customer: "cus_123",
      status: "active",
      price_id: "price_personal",
      current_period_end: 1.week.from_now.to_i,
      cancel_at_period_end: true
    )

    user.sync_stripe_subscription!(subscription)
    user.reload

    assert_equal "cus_123", user.stripe_customer_id
    assert_equal "sub_123", user.stripe_subscription_id
    assert_equal "active", user.stripe_subscription_status
    assert_equal "price_personal", user.stripe_price_id
    assert user.subscription_current_period_end.present?
    assert user.subscription_cancel_at_period_end
  end

  test "sync_stripe_subscription! preserves existing billing details when payload is partial" do
    current_period_end = 3.days.from_now.change(usec: 0)
    user = User.create!(
      email: "partial@example.com",
      password: "password123",
      name: "Partial Billing User",
      stripe_customer_id: "cus_existing",
      stripe_subscription_id: "sub_existing",
      stripe_subscription_status: "trialing",
      stripe_price_id: "price_existing",
      subscription_current_period_end: current_period_end,
      subscription_cancel_at_period_end: true
    )

    subscription = { status: "past_due" }

    user.sync_stripe_subscription!(subscription)
    user.reload

    assert_equal "cus_existing", user.stripe_customer_id
    assert_equal "sub_existing", user.stripe_subscription_id
    assert_equal "past_due", user.stripe_subscription_status
    assert_equal "price_existing", user.stripe_price_id
    assert_equal current_period_end, user.subscription_current_period_end
    assert user.subscription_cancel_at_period_end
  end

  test "mark_subscription_canceled! preserves customer and marks inactive" do
    user = User.create!(
      email: "cancel@example.com",
      password: "password123",
      name: "Cancel User",
      stripe_customer_id: "cus_existing",
      stripe_subscription_id: "sub_existing",
      stripe_subscription_status: "active",
      stripe_price_id: "price_personal"
    )

    subscription = stripe_subscription(
      id: "sub_existing",
      customer: "cus_existing",
      status: "canceled",
      price_id: "price_personal",
      current_period_end: Time.current.to_i,
      cancel_at_period_end: false
    )

    user.mark_subscription_canceled!(subscription)
    user.reload

    assert_equal "canceled", user.stripe_subscription_status
    assert_not user.subscription_active?
    assert_equal "cus_existing", user.stripe_customer_id
    assert_equal "sub_existing", user.stripe_subscription_id
  end

  test "mark_subscription_canceled! without payload preserves identifiers and period end" do
    current_period_end = 5.days.from_now.change(usec: 0)
    user = User.create!(
      email: "cancel-without-payload@example.com",
      password: "password123",
      name: "Cancel Without Payload",
      stripe_customer_id: "cus_existing",
      stripe_subscription_id: "sub_existing",
      stripe_subscription_status: "past_due",
      stripe_price_id: "price_existing",
      subscription_current_period_end: current_period_end,
      subscription_cancel_at_period_end: true
    )

    user.mark_subscription_canceled!
    user.reload

    assert_equal "cus_existing", user.stripe_customer_id
    assert_equal "sub_existing", user.stripe_subscription_id
    assert_equal "canceled", user.stripe_subscription_status
    assert_equal "price_existing", user.stripe_price_id
    assert_equal current_period_end, user.subscription_current_period_end
    assert_not user.subscription_cancel_at_period_end
  end

  private

  def google_auth_hash(email:, uid:, name: "OAuth User", email_verified: true)
    OmniAuth::AuthHash.new(
      provider: "google_oauth2",
      uid: uid,
      info: {
        email: email,
        email_verified: email_verified,
        name: name
      },
      extra: { raw_info: { sub: uid } }
    )
  end
end
