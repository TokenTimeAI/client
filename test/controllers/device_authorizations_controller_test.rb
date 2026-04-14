require "test_helper"

class DeviceAuthorizationsControllerTest < ActionDispatch::IntegrationTest
  test "show requires sign in" do
    authorization = DeviceAuthorization.create_pending!

    get "/device/#{authorization.user_code}"

    assert_response :redirect
    assert_redirected_to new_user_session_path
  end

  test "approve requires sign in" do
    authorization = DeviceAuthorization.create_pending!

    post "/device/#{authorization.user_code}/approve"

    assert_response :redirect
    assert_redirected_to new_user_session_path
  end

  test "approve creates an api key for the signed in user" do
    authorization = DeviceAuthorization.create_pending!(machine_name: "office-mac")
    user = User.create!(
      email: "device-approval@example.com",
      password: "password123",
      password_confirmation: "password123",
      name: "Approver"
    )

    sign_in user

    assert_difference("ApiKey.count", 1) do
      post "/device/#{authorization.user_code}/approve"
    end

    assert_redirected_to device_authorization_path(authorization.user_code)

    authorization.reload
    assert_equal "approved", authorization.status
    assert_equal user, authorization.user
    assert_equal "Local heartbeat daemon approved.", flash[:notice]
    assert_equal "Local daemon - office-mac", authorization.api_key.name
  end
end
