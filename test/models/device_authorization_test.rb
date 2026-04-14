require "test_helper"

class DeviceAuthorizationTest < ActiveSupport::TestCase
  test "create_pending generates codes and stores only device code digest" do
    authorization = DeviceAuthorization.create_pending!(machine_name: "mbp")

    assert authorization.device_code.present?
    assert authorization.user_code.present?
    assert_equal DeviceAuthorization.digest_device_code(authorization.device_code), authorization.device_code_digest
    assert_nil authorization.attributes["device_code"]
    assert authorization.expires_at.future?
    assert_equal "pending", authorization.status
  end

  test "find_by_device_code authenticates using the raw code" do
    authorization = DeviceAuthorization.create_pending!

    assert_equal authorization, DeviceAuthorization.find_by_device_code(authorization.device_code)
    assert_nil DeviceAuthorization.find_by_device_code("missing")
  end

  test "expire_if_needed marks expired authorizations" do
    authorization = DeviceAuthorization.create_pending!(expires_in: -1.minute)

    assert authorization.expired?
    assert authorization.expire_if_needed!
    assert_equal "expired", authorization.reload.status
  end

  test "claim returns payload once and marks authorization claimed" do
    user, api_key = create_user_with_api_key(email: "device-auth-claim@example.com", name: "Device User")
    authorization = DeviceAuthorization.create_pending!(machine_name: "workstation")
    authorization.approve!(user: user, api_key: api_key)

    payload = authorization.claim!(api_base_url: "http://www.example.com/api/v1")

    assert_equal api_key.key, payload[:api_key]
    assert_equal "http://www.example.com/api/v1", payload[:api_base_url]
    assert_equal user.id, payload.dig(:user, :id)
    assert_equal "claimed", authorization.reload.status
    assert_raises(ActiveRecord::RecordInvalid) do
      authorization.claim!(api_base_url: "http://www.example.com/api/v1")
    end
  end
end
