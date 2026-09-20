import { useEffect, useRef } from 'react';
import * as Blockly from 'blockly';
import { javascriptGenerator } from 'blockly/javascript';

// ── Custom Block Definitions ──────────────────────────────────────────────────

function defineBlocks() {
  if (Blockly.Blocks['airouter_main']) return; // Prevent re-definition

  Blockly.Blocks['airouter_main'] = {
    init: function() {
      this.appendDummyInput().appendField("AI Router Rules");
      this.appendStatementInput("RULES").setCheck("Rule");
      this.setColour(230);
      this.setTooltip("The main container for AI routing rules");
      this.setDeletable(false); // don't allow deleting the main block
    }
  };

  javascriptGenerator.forBlock['airouter_main'] = (block) => {
    return javascriptGenerator.statementToCode(block, 'RULES');
  };

  Blockly.Blocks['airouter_rule'] = {
    init: function() {
      this.appendStatementInput("CONDITION")
          .setCheck("Condition")
          .appendField("When");
      this.appendStatementInput("ACTION")
          .setCheck("Action")
          .appendField("Route To");
      this.setPreviousStatement(true, "Rule");
      this.setNextStatement(true, "Rule");
      this.setColour(210);
      this.setTooltip("A routing rule");
    }
  };

  javascriptGenerator.forBlock['airouter_rule'] = (block) => {
    let conditionStr = javascriptGenerator.statementToCode(block, 'CONDITION').trim();
    let actionStr = javascriptGenerator.statementToCode(block, 'ACTION').trim();
    if (!conditionStr) conditionStr = `"condition": "always"`;
    if (!actionStr) actionStr = `"provider": "openai", "model": ""`;
    
    return `{ ${conditionStr}, ${actionStr} },`;
  };

  Blockly.Blocks['airouter_always'] = {
    init: function() {
      this.appendDummyInput().appendField("Always");
      this.setPreviousStatement(true, "Condition");
      this.setColour(160);
    }
  };

  javascriptGenerator.forBlock['airouter_always'] = () => `"condition": "always"`;

  Blockly.Blocks['airouter_if_model_contains'] = {
    init: function() {
      this.appendDummyInput()
          .appendField("If model contains")
          .appendField(new Blockly.FieldTextInput(""), "TEXT");
      this.setPreviousStatement(true, "Condition");
      this.setColour(160);
    }
  };

  javascriptGenerator.forBlock['airouter_if_model_contains'] = (block) => {
    const text = block.getFieldValue('TEXT');
    return `"condition": "if_model_contains:${text}"`;
  };

  Blockly.Blocks['airouter_fallback'] = {
    init: function() {
      this.appendDummyInput().appendField("Fallback");
      this.setPreviousStatement(true, "Condition");
      this.setColour(160);
    }
  };

  javascriptGenerator.forBlock['airouter_fallback'] = () => `"condition": "fallback"`;

  Blockly.Blocks['airouter_action'] = {
    init: function() {
      this.appendDummyInput()
          .appendField("Provider")
          .appendField(new Blockly.FieldDropdown([
            ["OpenAI", "openai"],
            ["Anthropic", "anthropic"],
            ["Custom", "custom"]
          ]), "PROVIDER")
          .appendField("Model")
          .appendField(new Blockly.FieldTextInput(""), "MODEL");
      this.setPreviousStatement(true, "Action");
      this.setColour(20);
    }
  };

  javascriptGenerator.forBlock['airouter_action'] = (block) => {
    const provider = block.getFieldValue('PROVIDER');
    const model = block.getFieldValue('MODEL');
    return `"provider": "${provider}", "model": "${model}"`;
  };
}

// ── Toolbox ───────────────────────────────────────────────────────────────────

const TOOLBOX = {
  kind: 'categoryToolbox',
  contents: [
    {
      kind: 'category',
      name: 'Rules',
      colour: '210',
      contents: [
        { kind: 'block', type: 'airouter_rule' },
      ],
    },
    {
      kind: 'category',
      name: 'Conditions',
      colour: '160',
      contents: [
        { kind: 'block', type: 'airouter_always' },
        { kind: 'block', type: 'airouter_if_model_contains' },
        { kind: 'block', type: 'airouter_fallback' },
      ],
    },
    {
      kind: 'category',
      name: 'Actions',
      colour: '20',
      contents: [
        { kind: 'block', type: 'airouter_action' },
      ],
    },
  ],
};

// ── Component ─────────────────────────────────────────────────────────────────

interface AIRouterBlocklyProps {
  onChange: (rules: any[]) => void;
  initialRules?: any[]; initialXml?: string;
}

export function generateJSON(ws: Blockly.WorkspaceSvg): any[] {
  const mainBlock = ws.getTopBlocks(false).find(b => b.type === 'airouter_main');
  if (!mainBlock) return [];
  const rulesCode = javascriptGenerator.statementToCode(mainBlock, 'RULES');
  try {
    const jsonStr = `[${rulesCode.trim().replace(/,$/, '')}]`;
    return JSON.parse(jsonStr);
  } catch (e) {
    console.error("Failed to parse AI router JSON:", e, rulesCode);
    return [];
  }
}

export function AIRouterBlockly({ onChange, initialXml }: AIRouterBlocklyProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const workspaceRef = useRef<Blockly.WorkspaceSvg | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    defineBlocks();

    const ws = Blockly.inject(containerRef.current, {
      toolbox: TOOLBOX,
      theme: {
        name: 'zcdns',
        base: Blockly.Themes.Classic,
        componentStyles: {
          workspaceBackgroundColour: '#f8fafc',
          toolboxBackgroundColour: '#1e293b',
          toolboxForegroundColour: '#f8fafc',
          flyoutBackgroundColour: '#0f172a',
          flyoutForegroundColour: '#e2e8f0',
          flyoutOpacity: 0.95,
          scrollbarColour: '#94a3b8',
          insertionMarkerColour: '#22c55e',
          insertionMarkerOpacity: 0.6,
        },
      } as unknown as Blockly.Theme,
      trashcan: true,
      zoom: { controls: true, wheel: true, startScale: 0.9, maxScale: 2, minScale: 0.4 },
      grid: { spacing: 20, length: 3, colour: '#e2e8f0', snap: true },
      scrollbars: true,
      move: { scrollbars: true, drag: true, wheel: true },
    });

    workspaceRef.current = ws;

    if (initialXml) {
      try {
        const dom = Blockly.utils.xml.textToDom(initialXml);
        Blockly.Xml.domToWorkspace(dom, ws);
      } catch { /* ignore */ }
    } else {
      const starterXml = `
        <xml>
          <block type="airouter_main" x="30" y="30" deletable="false">
            <statement name="RULES">
              <block type="airouter_rule">
                <statement name="CONDITION">
                  <block type="airouter_always"></block>
                </statement>
                <statement name="ACTION">
                  <block type="airouter_action">
                    <field name="PROVIDER">openai</field>
                    <field name="MODEL">gpt-4o-mini</field>
                  </block>
                </statement>
              </block>
            </statement>
          </block>
        </xml>`;
      const dom = Blockly.utils.xml.textToDom(starterXml);
      Blockly.Xml.domToWorkspace(dom, ws);
    }

    const listener = () => {
      onChange(generateJSON(ws));
    };
    ws.addChangeListener(listener);
    
    // Trigger an initial onChange so state isn't empty on load
    listener();

    return () => {
      ws.removeChangeListener(listener);
      ws.dispose();
      workspaceRef.current = null;
    };
  }, [onChange, initialXml]);

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-gray-500">
        Drag blocks from the left panel to configure your AI Router logic.
      </p>
      <div
        ref={containerRef}
        className="w-full rounded-lg border border-gray-200 overflow-hidden"
        style={{ height: '480px' }}
      />
    </div>
  );
}
