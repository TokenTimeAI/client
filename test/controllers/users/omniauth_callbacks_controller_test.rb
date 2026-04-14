require "test_helper"
require "omniauth"

class Users::OmniauthCallbacksControllerTest < ActionDispatch::IntegrationTest
  parallelize(workers: 1)

  setup do
    OmniAuth.config.test_mode = true
  end

  teardown do
    OmniAuth.config.mock_auth[:github] = nil
    OmniAuth.config.mock_auth[:google_oauth2] = nil
    OmniAuth.config.test_mode = false
  end

  test "github callback signs in an existing user with a verified email" do
    existing_user = User.create!(email: "octo@example.com", password: "password123", name: "Octo Cat")
    OmniAuth.config.mock_auth[:github] = OmniAuth::AuthHash.new(
      provider: "github",
      uid: "github-123",
      info: { email: existing_user.email, nickname: "octocat", name: "Octo Cat" },
      extra: { all_emails: [{ email: existing_user.email, verified: true, primary: true }] }
    )

    post user_github_omniauth_authorize_path
    follow_redirect! while response.redirect?

    assert_response :success
    assert_equal "github-123", existing_user.reload.uid
    assert_equal "github", existing_user.reload.provider
    assert_match(/Dashboard/, response.body)
    assert_match(/Successfully authenticated from GitHub account\./, response.body)
  end

  test "google callback creates a user" do
    OmniAuth.config.mock_auth[:google_oauth2] = OmniAuth::AuthHash.new(
      provider: "google_oauth2",
      uid: "google-abc",
      info: { email: "new-google@example.com", email_verified: true, name: "Google User" }
    )

    assert_difference("User.count", 1) do
      post user_google_oauth2_omniauth_authorize_path
      follow_redirect! while response.redirect?
    end

    assert_response :success
    created_user = User.find_by!(email: "new-google@example.com")
    assert_equal "google_oauth2", created_user.provider
    assert_equal "google-abc", created_user.uid
    assert_match(/Dashboard/, response.body)
  end
end
