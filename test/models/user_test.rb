require "test_helper"

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
    # ULID should only contain Crockford base32 characters
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
end
