require "test_helper"

class BillingControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user = User.create!(email: "subscriber@example.com", password: "password123", name: "Subscriber")
    sign_in @user
  end

  test "shows billing page" do
    get billing_path

    assert_response :success
    assert_match "$20.00/month", response.body
    assert_match "Need a team plan?", response.body
  end

  test "creates checkout session for signed-in user" do
    checkout_session = FakeUrl.new("https://checkout.stripe.test/session")
    customer = Struct.new(:id).new("cus_new")

    with_env("STRIPE_SECRET_KEY" => "sk_test_123", "STRIPE_PERSONAL_MONTHLY_PRICE_ID" => "price_123") do
      with_singleton_stub(Stripe::Customer, :create, value: customer) do
        with_singleton_stub(Stripe::Checkout::Session, :create, value: checkout_session) do
            post checkout_billing_path
        end
      end
    end

    assert_redirected_to "https://checkout.stripe.test/session"
    assert_equal "cus_new", @user.reload.stripe_customer_id
  end

  test "creates checkout session with existing stripe customer without creating another customer" do
    @user.update!(stripe_customer_id: "cus_existing")
    checkout_session = FakeUrl.new("https://checkout.stripe.test/existing-customer-session")

    with_env("STRIPE_SECRET_KEY" => "sk_test_123", "STRIPE_PERSONAL_MONTHLY_PRICE_ID" => "price_123") do
      with_singleton_stub(Stripe::Customer, :create, replacement: ->(*) { flunk "expected existing stripe customer to be reused" }) do
        with_singleton_stub(Stripe::Checkout::Session, :create, value: checkout_session) do
            post checkout_billing_path
        end
      end
    end

    assert_redirected_to "https://checkout.stripe.test/existing-customer-session"
    assert_equal "cus_existing", @user.reload.stripe_customer_id
  end

  test "redirects active subscribers to billing portal instead of new checkout" do
    @user.update!(stripe_subscription_status: "active")

    with_env("STRIPE_SECRET_KEY" => "sk_test_123", "STRIPE_PERSONAL_MONTHLY_PRICE_ID" => "price_123") do
      post checkout_billing_path
    end

    assert_redirected_to billing_path
    follow_redirect!
    assert_match "Manage it below", response.body
  end

  test "redirects trialing subscribers away from duplicate checkout" do
    @user.update!(stripe_subscription_status: "trialing")

    with_env("STRIPE_SECRET_KEY" => "sk_test_123", "STRIPE_PERSONAL_MONTHLY_PRICE_ID" => "price_123") do
      post checkout_billing_path
    end

    assert_redirected_to billing_path
    follow_redirect!
    assert_match "Manage it below", response.body
  end

  test "opens billing portal for existing customer" do
    @user.update!(stripe_customer_id: "cus_existing", stripe_subscription_status: "active")
    portal_session = FakeUrl.new("https://billing.stripe.test/session")

    with_env("STRIPE_SECRET_KEY" => "sk_test_123") do
      with_singleton_stub(Stripe::BillingPortal::Session, :create, value: portal_session) do
        post portal_billing_path
      end
    end

    assert_redirected_to "https://billing.stripe.test/session"
  end

  test "redirects to billing when customer is missing for portal" do
    with_env("STRIPE_SECRET_KEY" => "sk_test_123") do
      post portal_billing_path
    end

    assert_redirected_to billing_path
    follow_redirect!
    assert_match "Start a subscription first", response.body
  end
end
