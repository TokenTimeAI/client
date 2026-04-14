require "test_helper"

class Api::V1::SummariesControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user, @api_key = create_user_with_api_key
    @headers = { "Authorization" => "Bearer #{@api_key.key}" }

    # Create test heartbeats within the last hour
    now = Time.now
    @project = @user.projects.create!(name: "test-project")
    [0, 30, 60, 120, 180].each_with_index do |seconds_ago, i|
      @user.heartbeat_events.create!(
        agent_type: i.even? ? "claude_code" : "codex",
        entity: "file#{i}.rb",
        entity_type: "file",
        language: "Ruby",
        project: @project,
        time: (now - seconds_ago).to_f
      )
    end
  end

  test "returns summaries for date range" do
    today = Date.today.iso8601
    get "/api/v1/summaries?start=#{today}&end=#{today}", headers: @headers
    assert_response :success

    data = JSON.parse(response.body)
    assert data["data"].is_a?(Array)
    assert data["data"].first["grand_total"].present?
    assert data["cumulative_total"].present?
  end

  test "returns language breakdown" do
    today = Date.today.iso8601
    get "/api/v1/summaries?start=#{today}&end=#{today}", headers: @headers

    data = JSON.parse(response.body)
    languages = data["data"].flat_map { |d| d["languages"] }
    assert languages.any? { |l| l["name"] == "Ruby" }
  end

  test "returns agent breakdown in editors" do
    today = Date.today.iso8601
    get "/api/v1/summaries?start=#{today}&end=#{today}", headers: @headers

    data = JSON.parse(response.body)
    editors = data["data"].flat_map { |d| d["editors"] }
    agent_names = editors.map { |e| e["name"] }
    assert_includes agent_names, "claude_code"
  end
end
