require "test_helper"

class StripeWebhooksControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user = User.create!(email: "webhook@example.com", password: "password123", name: "Webhook User")
  end

  test "checkout session completed stores customer and subscription ids" do
    session = CheckoutSessionObject.new(
      mode: "subscription",
      client_reference_id: @user.id,
      customer: "cus_123",
      subscription: "sub_123",
      customer_email: @user.email
    )
    event = stripe_event("checkout.session.completed", session)

    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, value: event) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :success
    @user.reload
    assert_equal "cus_123", @user.stripe_customer_id
    assert_equal "sub_123", @user.stripe_subscription_id
  end

  test "checkout session completed falls back to customer email when client reference id is missing" do
    session = CheckoutSessionObject.new(
      mode: "subscription",
      customer: "cus_email_match",
      subscription: "sub_email_match",
      customer_email: @user.email
    )
    event = stripe_event("checkout.session.completed", session)

    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, value: event) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :success
    @user.reload
    assert_equal "cus_email_match", @user.stripe_customer_id
    assert_equal "sub_email_match", @user.stripe_subscription_id
  end

  test "customer subscription updated syncs billing state" do
    subscription = stripe_subscription(
      id: "sub_456",
      customer: "cus_456",
      status: "active",
      price_id: "price_456",
      current_period_end: 2.days.from_now.to_i,
      cancel_at_period_end: false,
      metadata: { "user_id" => @user.id }
    )
    event = stripe_event("customer.subscription.updated", subscription)

    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, value: event) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :success
    @user.reload
    assert_equal "cus_456", @user.stripe_customer_id
    assert_equal "sub_456", @user.stripe_subscription_id
    assert_equal "active", @user.stripe_subscription_status
    assert_equal "price_456", @user.stripe_price_id
    assert @user.subscription_active?
  end

  test "customer subscription updated can match an existing user by customer id without metadata" do
    @user.update!(stripe_customer_id: "cus_existing")

    subscription = stripe_subscription(
      id: "sub_existing_lookup",
      customer: "cus_existing",
      status: "past_due",
      price_id: "price_existing_lookup",
      current_period_end: 3.days.from_now.to_i,
      cancel_at_period_end: true,
      metadata: {}
    )
    event = stripe_event("customer.subscription.updated", subscription)

    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, value: event) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :success
    @user.reload
    assert_equal "sub_existing_lookup", @user.stripe_subscription_id
    assert_equal "past_due", @user.stripe_subscription_status
    assert_equal "price_existing_lookup", @user.stripe_price_id
    assert @user.subscription_active?
    assert @user.subscription_cancel_at_period_end
  end

  test "customer subscription deleted marks subscription canceled" do
    @user.update!(
      stripe_customer_id: "cus_789",
      stripe_subscription_id: "sub_789",
      stripe_subscription_status: "active",
      stripe_price_id: "price_789"
    )

    deleted_at = Time.current.change(usec: 0)
    subscription = stripe_subscription(
      id: "sub_789",
      customer: "cus_789",
      status: "canceled",
      price_id: "price_789",
      current_period_end: deleted_at.to_i,
      cancel_at_period_end: false,
      metadata: {}
    )
    event = stripe_event("customer.subscription.deleted", subscription)

    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, value: event) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :success
    @user.reload
    assert_equal "canceled", @user.stripe_subscription_status
    assert_not @user.subscription_active?
    assert_equal deleted_at, @user.subscription_current_period_end
    assert_not @user.subscription_cancel_at_period_end
  end

  test "invalid webhook signature returns bad request" do
    with_env("STRIPE_WEBHOOK_SECRET" => "whsec_test") do
      with_singleton_stub(Stripe::Webhook, :construct_event, replacement: ->(*) { raise Stripe::SignatureVerificationError.new("bad sig", "sig") }) do
        post "/stripe/webhooks", params: "{}", headers: { "HTTP_STRIPE_SIGNATURE" => "sig_test" }
      end
    end

    assert_response :bad_request
  end
end
