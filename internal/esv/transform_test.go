package esv

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestProcessPassageHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Stylized, same paragraph",
			input: `
<h2 id="test-h2" class="extra_text">Genesis 2:17\u201325</h2>
<p id="p01002017_01-1" class="virtual"><b class="verse-num" id="v01002017-1">&nbsp;17&nbsp;</b>but of the tree of the knowledge of good and evil you shall not eat, for in the day that you eat of it you shall surely die.\u201d</p>
<p id="p01002017_01-1"><b class="verse-num" id="v01002018-1">18</b>Then the LORD God said, \u201cIt is not good that the man should be alone; I will make him a helper fit for him.\u201d <b class="verse-num" id="v01002019-1">19</b>Now out of the ground the LORD God had formed every beast of the field and every bird of the heavens and brought them to the man to see what he would call them. And whatever the man called every living creature, that was its name. <b class="verse-num" id="v01002020-1">20</b>The man gave names to all livestock and to the birds of the heavens and to every beast of the field. But for Adam there was not found a helper fit for him. <b class="verse-num" id="v01002021-1">21</b>So the LORD God caused a deep sleep to fall upon the man, and while he slept took one of his ribs and closed up its place with flesh. <b class="verse-num" id="v01002022-1">22</b>And the rib that the LORD God had taken from the man he made into a woman and brought her to the man. <b class="verse-num" id="v01002023-1">23</b>Then the man said,</p>
<p class="block-indent"><section class="line-group">
<span id="p01002023_01-1" class="line">&nbsp;&nbsp;\u201cThis at last is bone of my bones</span><br /><span id="p01002023_01-1" class="indent line">&nbsp;&nbsp;&nbsp;&nbsp;and flesh of my flesh;</span><br /><span id="p01002023_01-1" class="line">&nbsp;&nbsp;she shall be called Woman,</span><br /><span id="p01002023_01-1" class="indent line">&nbsp;&nbsp;&nbsp;&nbsp;because she was taken out of Man.\u201d</span><br /><span class="end-line-group"></span>
</p><p id="p01002023_01-1" class="same-paragraph"><b class="verse-num" id="v01002024-1">24</b>Therefore a man shall leave his father and his mother and hold fast to his wife, and they shall become one flesh. <b class="verse-num">&nbsp;25&nbsp;</b>And the man and his wife were both naked and were not ashamed.</p>
<p>(<a href="http://www.esv.org" class="copyright">ESV</a>)</p>`,
			expected: "\n<h2 class=\"extra_text\">Genesis 2:17–25</h2>\n<p><span class=\"verse\" data-ref=\"01002017\"><b class=\"verse-num\">17</b>but of the tree of the knowledge of good and evil you shall not eat, for in the day that you eat of it you shall surely die.”</span></p>\n<p><span class=\"verse\" data-ref=\"01002018\"><b class=\"verse-num\">18</b>Then the LORD God said, “It is not good that the man should be alone; I will make him a helper fit for him.”</span><span class=\"verse\" data-ref=\"01002019\"><b class=\"verse-num\">19</b>Now out of the ground the LORD God had formed every beast of the field and every bird of the heavens and brought them to the man to see what he would call them. And whatever the man called every living creature, that was its name.</span><span class=\"verse\" data-ref=\"01002020\"><b class=\"verse-num\">20</b>The man gave names to all livestock and to the birds of the heavens and to every beast of the field. But for Adam there was not found a helper fit for him.</span><span class=\"verse\" data-ref=\"01002021\"><b class=\"verse-num\">21</b>So the LORD God caused a deep sleep to fall upon the man, and while he slept took one of his ribs and closed up its place with flesh.</span><span class=\"verse\" data-ref=\"01002022\"><b class=\"verse-num\">22</b>And the rib that the LORD God had taken from the man he made into a woman and brought her to the man.</span><span class=\"verse\" data-ref=\"01002023\"><b class=\"verse-num\">23</b>Then the man said,</span></p>\n<section class=\"line-group\">\n<span class=\"line verse\" data-ref=\"01002023\">“This at last is bone of my bones</span><br/><span class=\"indent line verse\" data-ref=\"01002023\">and flesh of my flesh;</span><br/><span class=\"line verse\" data-ref=\"01002023\">she shall be called Woman,</span><br/><span class=\"indent line verse\" data-ref=\"01002023\">because she was taken out of Man.”</span><br/>\n<p class=\"same-paragraph\"><span class=\"verse\" data-ref=\"01002024\"><b class=\"verse-num\">24</b>Therefore a man shall leave his father and his mother and hold fast to his wife, and they shall become one flesh. <b class=\"verse-num\"><span class=\"verse\" data-ref=\"01002024\">25</span></b>And the man and his wife were both naked and were not ashamed.</span></p>\n<p>(<a href=\"http://www.esv.org\" class=\"copyright\">ESV</a>)</p></section>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processPassageHTML(tt.input)
			if err != nil {
				t.Errorf("processPassageHTML() error = %v", err)
			}
			if diff := cmp.Diff(got, tt.expected); diff != "" {
				t.Errorf("processPassageHTML() mismatch (-want +got):\n%s", diff)
				t.Logf("GOT: %q", got)
			}
		})
	}
}
