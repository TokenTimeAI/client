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

  test "supports native session timing dimensions" do
    started_at = Time.zone.parse("2026-04-14 10:00:00 UTC")
    ended_at = Time.zone.parse("2026-04-14 10:10:00 UTC")

    hb = HeartbeatEvent.create!(
      user: @user,
      agent_type: "cosine",
      entity: "/Users/pz/w/cosine/apps/cli2",
      entity_type: "conversation",
      time: ended_at.to_f,
      session_started_at: started_at,
      session_ended_at: ended_at,
      session_duration_seconds: 600,
      agent_active_seconds: 420,
      human_active_seconds: 120,
      idle_seconds: 60
    )

    assert_equal started_at, hb.session_started_at
    assert_equal ended_at, hb.session_ended_at
    assert_equal 600, hb.session_duration_seconds
    assert_equal 420, hb.agent_active_seconds
    assert_equal 120, hb.human_active_seconds
    assert_equal 60, hb.idle_seconds
  end
end
