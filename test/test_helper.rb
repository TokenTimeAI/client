ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
require "devise/test/integration_helpers"

module ActiveSupport
  class TestCase
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
