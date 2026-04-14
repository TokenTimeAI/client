require "test_helper"

class Api::V1::UsersControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user, @api_key = create_user_with_api_key(name: "Alice Ttime")
    @headers = { "Authorization" => "Bearer #{@api_key.key}" }
  end

  test "returns current user" do
    get "/api/v1/users/current", headers: @headers
    assert_response :success

    data = JSON.parse(response.body)["data"]
    assert_equal @user.id, data["id"]
    assert_equal @user.email, data["email"]
    assert_equal "Alice Ttime", data["name"]
    assert_equal "UTC", data["timezone"]
  end

  test "returns 401 without auth" do
    get "/api/v1/users/current"
    assert_response :unauthorized
  end
end
