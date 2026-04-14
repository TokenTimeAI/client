require "test_helper"

class Api::V1::HeartbeatsControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user, @api_key = create_user_with_api_key
    @headers = {
      "Authorization" => "Bearer #{@api_key.key}",
      "Content-Type" => "application/json"
    }
  end

  test "returns 401 without auth" do
    post "/api/v1/heartbeats", params: {}.to_json,
         headers: { "Content-Type" => "application/json" }
    assert_response :unauthorized
  end

  test "creates a single heartbeat" do
    payload = {
      entity: "app/models/user.rb",
      type: "file",
      language: "Ruby",
      project: "test-project",
      agent_type: "claude_code",
      time: Time.now.to_f,
      is_write: true,
      tokens_used: 1000,
      lines_added: 20,
      branch: "main"
    }

    assert_difference("HeartbeatEvent.count", 1) do
      post "/api/v1/heartbeats", params: payload.to_json, headers: @headers
    end

    assert_response :created
    data = JSON.parse(response.body)
    assert_equal "app/models/user.rb", data.dig("data", "entity")
    assert_equal "claude_code", data.dig("data", "agent_type")
    assert_equal "Ruby", data.dig("data", "language")
    assert_equal "test-project", data.dig("data", "project")
    assert_equal 26, data.dig("data", "id").length  # ULID length
  end

  test "creates project if it doesn't exist" do
    payload = {
      entity: "main.py",
      type: "file",
      agent_type: "codex",
      time: Time.now.to_f,
      project: "brand-new-project"
    }

    assert_difference("Project.count", 1) do
      post "/api/v1/heartbeats", params: payload.to_json, headers: @headers
    end

    assert_response :created
    assert @user.projects.find_by(name: "brand-new-project")
  end

  test "bulk creates multiple heartbeats" do
    now = Time.now.to_f
    payload = [
      { entity: "a.rb", type: "file", agent_type: "claude_code", time: now },
      { entity: "b.rb", type: "file", agent_type: "codex", time: now + 30 },
      { entity: "c.rb", type: "file", agent_type: "cursor", time: now + 60 }
    ]

    assert_difference("HeartbeatEvent.count", 3) do
      post "/api/v1/heartbeats/bulk", params: payload.to_json, headers: @headers
    end

    assert_response :created
    data = JSON.parse(response.body)
    assert_equal 3, data["responses"].length
    data["responses"].each do |status, _|
      assert_equal 201, status
    end
  end

  test "bulk returns errors for invalid heartbeats" do
    payload = [
      { agent_type: "claude_code", time: Time.now.to_f }  # missing entity
    ]

    post "/api/v1/heartbeats/bulk", params: payload.to_json, headers: @headers

    assert_response :created  # bulk always returns 201
    data = JSON.parse(response.body)
    assert_equal 400, data["responses"][0][0]
  end

  test "supports WakaTime-compatible route" do
    payload = { entity: "main.rb", type: "file", agent_type: "claude_code", time: Time.now.to_f }
    assert_difference("HeartbeatEvent.count", 1) do
      post "/api/v1/users/current/heartbeats", params: payload.to_json, headers: @headers
    end
    assert_response :created
  end

  test "supports Basic auth (WakaTime-style)" do
    credentials = Base64.strict_encode64("#{@api_key.key}:unused")
    headers = {
      "Authorization" => "Basic #{credentials}",
      "Content-Type" => "application/json"
    }

    payload = { entity: "test.rb", type: "file", agent_type: "claude_code", time: Time.now.to_f }
    assert_difference("HeartbeatEvent.count", 1) do
      post "/api/v1/heartbeats", params: payload.to_json, headers: headers
    end
    assert_response :created
  end
end
