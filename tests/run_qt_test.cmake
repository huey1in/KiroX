# QtTest's default Windows console logger can bypass redirected stdout.
# An explicit text log preserves diagnostics for CTest on every platform.
file(REMOVE "${TEST_LOG}")
execute_process(COMMAND "${TEST_EXECUTABLE}" -o "${TEST_LOG},txt" RESULT_VARIABLE test_result)
if(EXISTS "${TEST_LOG}")
  file(READ "${TEST_LOG}" test_output)
  message("${test_output}")
endif()
if(NOT test_result STREQUAL "0")
  message(FATAL_ERROR "QtTest exited with ${test_result}")
endif()
