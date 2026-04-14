ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
require "devise/test/integration_helpers"

module TestStubHelpers
  UNSET_STUB_VALUE = Object.new.freeze

  def with_singleton_stub(object, method_name, value: UNSET_STUB_VALUE, replacement: nil)
    singleton_class = class << object
      self
    end

    original_method = singleton_class.instance_method(method_name)
    original_visibility =
      if singleton_class.private_method_defined?(method_name)
        :private
      elsif singleton_class.protected_method_defined?(method_name)
        :protected
      else
        :public
      end

    implementation =
      if replacement
        replacement
      elsif value.equal?(UNSET_STUB_VALUE)
        ->(*) { nil }
      elsif value.respond_to?(:call)
        value
      else
        ->(*) { value }
      end

    singleton_class.define_method(method_name) do |*args, **kwargs, &block|
      if kwargs.empty?
        implementation.call(*args, &block)
      else
        implementation.call(*args, **kwargs, &block)
      end
    end

    yield
  ensure
    singleton_class.define_method(method_name, original_method)
    singleton_class.send(original_visibility, method_name)
  end

  def with_env(updates)
    original = {}

    updates.each do |key, value|
      original[key] = ENV.key?(key) ? ENV[key] : UNSET_STUB_VALUE
      value.nil? ? ENV.delete(key) : ENV[key] = value
    end

    yield
  ensure
    original.each do |key, value|
      value.equal?(UNSET_STUB_VALUE) ? ENV.delete(key) : ENV[key] = value
    end
  end
end

module StripeTestHelpers
  FakeUrl = Struct.new(:url)
  SubscriptionItem = Struct.new(:price)
  SubscriptionItems = Struct.new(:data)
  Price = Struct.new(:id)
  SubscriptionObject = Struct.new(:id, :customer, :status, :items, :current_period_end, :cancel_at_period_end, :metadata, keyword_init: true)
  CheckoutSessionObject = Struct.new(:mode, :client_reference_id, :customer, :subscription, :customer_details, :customer_email, keyword_init: true)
  EventData = Struct.new(:object)
  Event = Struct.new(:type, :data)

  def stripe_price(id)
    Price.new(id)
  end

  def stripe_subscription_item(price_id)
    SubscriptionItem.new(stripe_price(price_id))
  end

  def stripe_subscription(
    id:,
    customer:,
    status:,
    price_id:,
    current_period_end:,
    cancel_at_period_end:,
    metadata: {}
  )
    SubscriptionObject.new(
      id: id,
      customer: customer,
      status: status,
      items: SubscriptionItems.new([ stripe_subscription_item(price_id) ]),
      current_period_end: current_period_end,
      cancel_at_period_end: cancel_at_period_end,
      metadata: metadata
    )
  end

  def stripe_event(type, object)
    Event.new(type, EventData.new(object))
  end
end

class ActionDispatch::IntegrationTest
  include Devise::Test::IntegrationHelpers
  include StripeTestHelpers
  include TestStubHelpers
end

module ActiveSupport
  class TestCase
    include StripeTestHelpers
    include TestStubHelpers

    # Run tests in parallel with specified workers
    parallelize(workers: :number_of_processors)

    # Setup all fixtures in test/fixtures/*.yml for all tests
    fixtures :all

    # Helper to create a user with an API key for testing
    def create_user_with_api_key(email: "test@example.com", name: "Test User")
      user = User.create!(
        email: email,
        password: "password123",
        password_confirmation: "password123",
        name: name
      )
      api_key = user.api_keys.create!(name: "Test Key")
      [ user, api_key ]
    end
  end
end

class ActionDispatch::IntegrationTest
  include Devise::Test::IntegrationHelpers
end
