require "test_helper"

class Api::V1::DeviceAuthorizationsControllerTest < ActionDispatch::IntegrationTest
  test "creates a pending device authorization" do
    post "/api/v1/device_authorizations", params: { machine_name: "work-mac" }

    assert_response :created

    body = JSON.parse(response.body)
    authorization = DeviceAuthorization.find_by!(user_code: body["user_code"])

    assert body["device_code"].present?
    assert_equal "work-mac", authorization.machine_name
    assert_equal "pending", authorization.status
    assert_equal "http://www.example.com/device/#{authorization.user_code}", body["verification_uri"]
    assert_equal 5, body["interval"]
    assert body["expires_in"] > 0
    assert_nil DeviceAuthorization.find_by(device_code_digest: body["device_code"])
  end

  test "poll returns authorization_pending while request is pending" do
    authorization = DeviceAuthorization.create_pending!

    post "/api/v1/device_authorizations/poll", params: { device_code: authorization.device_code }

    assert_response :bad_request
    assert_equal({ "error" => "authorization_pending" }, JSON.parse(response.body))
  end

  test "poll returns api key payload once after approval" do
    user, api_key = create_user_with_api_key(email: "device-auth-approved@example.com", name: "Approved User")
    authorization = DeviceAuthorization.create_pending!(machine_name: "studio")
    authorization.approve!(user: user, api_key: api_key)

    post "/api/v1/device_authorizations/poll", params: { device_code: authorization.device_code }

    assert_response :success

    body = JSON.parse(response.body)
    assert_equal api_key.key, body["api_key"]
    assert_equal "http://www.example.com/api/v1", body["api_base_url"]
    assert_equal user.id, body.dig("user", "id")
    assert_equal user.email, body.dig("user", "email")
    assert_equal user.display_name, body.dig("user", "name")
    assert_equal "claimed", authorization.reload.status
  end

  test "poll returns expired_token for expired authorizations" do
    authorization = DeviceAuthorization.create_pending!(expires_in: -1.minute)

    post "/api/v1/device_authorizations/poll", params: { device_code: authorization.device_code }

    assert_response :bad_request
    assert_equal({ "error" => "expired_token" }, JSON.parse(response.body))
    assert_equal "expired", authorization.reload.status
  end

  test "poll only returns the api key once" do
    user, api_key = create_user_with_api_key(email: "device-auth-once@example.com", name: "One Time User")
    authorization = DeviceAuthorization.create_pending!
    authorization.approve!(user: user, api_key: api_key)

    post "/api/v1/device_authorizations/poll", params: { device_code: authorization.device_code }
    assert_response :success

    post "/api/v1/device_authorizations/poll", params: { device_code: authorization.device_code }

    assert_response :gone
    assert_equal({ "error" => "authorization_claimed" }, JSON.parse(response.body))
  end
end
