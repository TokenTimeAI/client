require "test_helper"

class HeartbeatEventTest < ActiveSupport::TestCase
  setup do
    @user = User.create!(email: "hbtest@example.com", password: "password123", name: "HB Test")
  end

  test "is valid with required fields" do
    hb = HeartbeatEvent.new(
      user: @user,
      agent_type: "claude_code",
      entity: "app/models/user.rb",
      entity_type: "file",
      time: Time.now.to_f
    )
    assert hb.valid?
  end

  test "requires entity" do
    hb = HeartbeatEvent.new(user: @user, agent_type: "claude_code", time: Time.now.to_f)
    assert_not hb.valid?
    assert_includes hb.errors[:entity], "can't be blank"
  end

  test "gets a ULID id on create" do
    hb = HeartbeatEvent.create!(
      user: @user,
      agent_type: "codex",
      entity: "main.py",
      entity_type: "file",
      time: Time.now.to_f
    )
    assert hb.id.present?
    assert_equal 26, hb.id.length
  end

  test "timestamp returns time as UTC Time object" do
    ts = 1_700_000_000.0
    hb = HeartbeatEvent.new(time: ts)
    assert_equal Time.at(ts).utc, hb.timestamp
  end

  test "for_period scope filters by time range" do
    now = Time.now.to_f
    old_hb = HeartbeatEvent.create!(
      user: @user, agent_type: "codex", entity: "old.rb", entity_type: "file",
      time: (Time.now - 10.days).to_f
    )
    recent_hb = HeartbeatEvent.create!(
      user: @user, agent_type: "codex", entity: "recent.rb", entity_type: "file",
      time: now
    )

    results = @user.heartbeat_events.for_period(1.hour.ago, Time.now)
    assert_includes results, recent_hb
    assert_not_includes results, old_hb
  end
end
