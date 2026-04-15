require "test_helper"

class DashboardControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user = User.create!(email: "dashboard@example.com", password: "password123", name: "Dashboard User")
    @project_a = @user.projects.create!(name: "Project A")
    @project_b = @user.projects.create!(name: "Project B")
    sign_in @user

    now = Time.zone.now.change(hour: 10, min: 0, sec: 0)

    @user.heartbeat_events.create!(
      entity: "a.rb",
      entity_type: "file",
      agent_type: "claude_code",
      language: "Ruby",
      project: @project_a,
      time: now.to_f,
      session_duration_seconds: 600,
      agent_active_seconds: 300,
      human_active_seconds: 240
    )

    @user.heartbeat_events.create!(
      entity: "b.go",
      entity_type: "file",
      agent_type: "codex",
      language: "Go",
      project: @project_b,
      time: (now + 60).to_f,
      session_duration_seconds: 900,
      agent_active_seconds: 540,
      human_active_seconds: 180
    )
  end

  test "dashboard applies project filters consistently across visible sections" do
    get dashboard_path, params: { date_range: "today", project_id: @project_a.id }

    assert_response :success
    assert_match(/Total Events<\/p>\s*<p class="text-2xl font-bold text-white">1<\/p>/, response.body)
    assert_match(/Most Active Project<\/p>\s*<p class="text-lg font-semibold text-white">Project A<\/p>/, response.body)
    assert_match(/Languages<\/h2>.*Ruby/m, response.body)
    assert_no_match(/Languages<\/h2>.*Go/m, response.body)
    assert_match(/Agents<\/h2>.*Claude code/m, response.body)
    assert_no_match(/Agents<\/h2>.*Codex/m, response.body)
    assert_match(/Recent Activity.*a\.rb/m, response.body)
    assert_no_match(/Recent Activity.*b\.go/m, response.body)
  end
end
