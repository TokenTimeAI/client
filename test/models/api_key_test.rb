require "test_helper"

class ApiKeyTest < ActiveSupport::TestCase
  test "generates a key on create" do
    user = User.create!(email: "apikeytest@example.com", password: "password123", name: "Test")
    api_key = user.api_keys.create!(name: "My Key")

    assert api_key.key.present?
    assert api_key.key.start_with?("tt_")
    assert_equal 67, api_key.key.length # "tt_" + 64 hex chars
  end

  test "is active by default" do
    user = User.create!(email: "apikeytest2@example.com", password: "password123", name: "Test")
    api_key = user.api_keys.create!(name: "My Key")

    assert api_key.active?
  end

  test "authenticate returns user for valid key" do
    user = User.create!(email: "apikeytest3@example.com", password: "password123", name: "Test")
    api_key = user.api_keys.create!(name: "My Key")

    authenticated_user = ApiKey.authenticate(api_key.key)
    assert_equal user, authenticated_user
  end

  test "authenticate returns nil for invalid key" do
    assert_nil ApiKey.authenticate("invalid_key")
  end

  test "authenticate returns nil for revoked key" do
    user = User.create!(email: "apikeytest4@example.com", password: "password123", name: "Test")
    api_key = user.api_keys.create!(name: "My Key")
    api_key.update!(active: false)

    assert_nil ApiKey.authenticate(api_key.key)
  end
end
